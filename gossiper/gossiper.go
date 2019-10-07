package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/dedis/protobuf"

	"github.com/mikanikos/Peerster/helpers"
)

var maxBufferSize = 4096
var modeTypes = []string{"simple", "rumor", "status", "client"}
var rumorTimeout = 10
var antiEntropyTimeout = 10

// MutexPeers struct
type MutexPeers struct {
	Peers []*net.UDPAddr
	Mutex sync.RWMutex
}

// PacketsStorage struct
type PacketsStorage struct {
	OriginPacketsMap map[string]map[uint32]*helpers.ExtendedGossipPacket
	Mutex            sync.RWMutex
}

// MutexSequenceID struct
type MutexSequenceID struct {
	ID    uint32
	Mutex sync.RWMutex
}

// Gossiper struct
type Gossiper struct {
	name           string
	clientData     *helpers.NetworkData
	gossiperData   *helpers.NetworkData
	peers          MutexPeers
	simpleMode     bool
	originPackets  PacketsStorage
	seqID          MutexSequenceID
	statusChannels map[string]chan *helpers.ExtendedGossipPacket
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, simple bool) *Gossiper {
	addressGossiper, err := net.ResolveUDPAddr("udp4", address)
	helpers.ErrorCheck(err)
	connGossiper, err := net.ListenUDP("udp4", addressGossiper)
	helpers.ErrorCheck(err)
	addressUI, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)
	connUI, err := net.ListenUDP("udp4", addressUI)
	helpers.ErrorCheck(err)

	peers := make([]*net.UDPAddr, 0)
	for _, peer := range peersList {
		addressPeer, err := net.ResolveUDPAddr("udp4", peer)
		helpers.ErrorCheck(err)
		peers = append(peers, addressPeer)
	}

	originPackets := make(map[string]map[uint32]*helpers.ExtendedGossipPacket)
	//originPackets[name] = make(map[uint32]*helpers.ExtendedGossipPacket)

	return &Gossiper{
		name:           name,
		clientData:     &helpers.NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData:   &helpers.NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peers:          MutexPeers{Peers: peers},
		originPackets:  PacketsStorage{OriginPacketsMap: originPackets}, //MutexOriginPackets{OriginPacketsMap: originMessagesMap},
		simpleMode:     simple,
		seqID:          MutexSequenceID{ID: 1},
		statusChannels: make(map[string]chan *helpers.ExtendedGossipPacket),
	}
}

func initializeChannels(modeTypes []string) (channels map[string]chan *helpers.ExtendedGossipPacket) {
	channels = make(map[string]chan *helpers.ExtendedGossipPacket)
	for _, t := range modeTypes {
		channels[t] = make(chan *helpers.ExtendedGossipPacket)
	}
	return channels
}

// Run function
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UTC().UnixNano())

	channels := initializeChannels(modeTypes)

	go gossiper.handleConnectionClient(channels["client"])
	go gossiper.getPacket(gossiper.clientData, channels)

	if gossiper.simpleMode {
		go gossiper.handleConnectionSimple(channels["simple"])
	} else {
		go gossiper.handleConnectionStatus(channels["status"])
		go gossiper.handleConnectionRumor(channels["rumor"])
		go gossiper.startAntiEntropy()
	}

	gossiper.getPacket(gossiper.gossiperData, channels)

}

func (gossiper *Gossiper) startAntiEntropy() {
	timer := time.NewTicker(time.Duration(antiEntropyTimeout) * time.Second)
	for {
		select {
		case <-timer.C:
			peersCopy := gossiper.peers.getPeersAtomic()
			indexPeer := rand.Intn(len(peersCopy))
			randomPeer := peersCopy[indexPeer]
			//fmt.Println("Sending periodic status to " + randomPeer.String())
			gossiper.sendStatusPacket(randomPeer)
		}
	}
}

func (gossiper *Gossiper) handleConnectionRumor(rumorChannel chan *helpers.ExtendedGossipPacket) {
	for extPacket := range rumorChannel {

		fmt.Println("RUMOR origin " + extPacket.Packet.Rumor.Origin + " from " + extPacket.SenderAddr.String() + " ID " + fmt.Sprint(extPacket.Packet.Rumor.ID) + " contents " + extPacket.Packet.Rumor.Text)
		gossiper.peers.addPeer(extPacket.SenderAddr)
		printPeers(gossiper.peers.getPeersAtomic())

		gossiper.originPackets.Mutex.Lock()

		messages, isPeerKnown := gossiper.originPackets.OriginPacketsMap[extPacket.Packet.Rumor.Origin]
		isMessageKnown := false
		if isPeerKnown {
			_, isMessageKnown = messages[extPacket.Packet.Rumor.ID]
		} else {
			gossiper.originPackets.OriginPacketsMap[extPacket.Packet.Rumor.Origin] = make(map[uint32]*helpers.ExtendedGossipPacket)
		}
		if !isMessageKnown {
			gossiper.originPackets.OriginPacketsMap[extPacket.Packet.Rumor.Origin][extPacket.Packet.Rumor.ID] = extPacket
		}
		gossiper.originPackets.Mutex.Unlock()
		gossiper.sendStatusPacket(extPacket.SenderAddr)
		if !isMessageKnown {
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func differenceString(a, b []*net.UDPAddr) []*net.UDPAddr {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.String()] = struct{}{}
	}
	var diff []*net.UDPAddr
	for _, x := range a {
		if _, found := mb[x.String()]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func (gossiper *Gossiper) startRumorMongering(extPacket *helpers.ExtendedGossipPacket) {
	coin := 1

	peersWithRumor := make([]*net.UDPAddr, 0)
	peersWithRumor = append(peersWithRumor, extPacket.SenderAddr)

	for coin == 1 {
		peers := gossiper.peers.getPeersAtomic()
		availablePeers := differenceString(peers, peersWithRumor)
		if len(availablePeers) == 0 {
			return
		}
		indexPeer := rand.Intn(len(availablePeers))
		randomPeer := availablePeers[indexPeer]
		peersWithRumor = append(peersWithRumor, randomPeer)

		fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
		wasStatusReceived := gossiper.sendRumorWithTimeout(extPacket.Packet, randomPeer)
		//gossiper.sendRumorWithTimeout(extPacket.Packet, randomPeer)

		if wasStatusReceived {
			coin = rand.Int() % 2
		}
	}
}

func (gossiper *Gossiper) sendStatusPacket(addr *net.UDPAddr) {
	myStatus := gossiper.createStatus()
	statusPacket := &helpers.StatusPacket{Want: myStatus}
	packet := &helpers.GossipPacket{Status: statusPacket}
	gossiper.sendPacket(packet, addr)
}

func (gossiper *Gossiper) sendRumorWithTimeout(packet *helpers.GossipPacket, peer *net.UDPAddr) bool {
	gossiper.statusChannels[peer.String()] = make(chan *helpers.ExtendedGossipPacket)

	gossiper.sendPacket(packet, peer)
	fmt.Println("MONGERING with " + peer.String())

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-gossiper.statusChannels[peer.String()]:
			return true
		case <-timer.C:
			return false
		}
	}
}

func (gossiper *Gossiper) createStatus() []helpers.PeerStatus {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	myStatus := make([]helpers.PeerStatus, 0)
	for origin, idMessages := range gossiper.originPackets.OriginPacketsMap {
		//fmt.Println(origin)
		//if origin != gossiper.name || includeMyself {
		var maxID uint32 = 0
		for id := range idMessages {
			//	fmt.Println(fmt.Sprint(id))
			if id > maxID {
				maxID = id
			}
		}
		myStatus = append(myStatus, helpers.PeerStatus{Identifier: origin, NextID: maxID + 1})
		//}
	}
	return myStatus
}

func (gossiper *Gossiper) getDifferenceStatus(myStatus, otherStatus []helpers.PeerStatus) []helpers.PeerStatus {
	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}
	difference := make([]helpers.PeerStatus, 0)
	for _, elem := range myStatus {
		//message, _ := gossiper.originPackets.OriginPacketsMap[elem.Identifier][elem.NextID-1]
		//if elem.NextID == 1 || message.SenderAddr.String() != otherAddr {
		id, isOriginKnown := originIDMap[elem.Identifier]
		if !isOriginKnown {
			difference = append(difference, helpers.PeerStatus{Identifier: elem.Identifier, NextID: 1})
		} else if elem.NextID > id {
			difference = append(difference, helpers.PeerStatus{Identifier: elem.Identifier, NextID: id})
		}
		//}
	}
	return difference
}

func (gossiper *Gossiper) getPacketsFromStatus(ps helpers.PeerStatus) []*helpers.GossipPacket {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	idMessages, _ := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
	maxID := ps.NextID
	for id := range idMessages {
		if id > maxID {
			maxID = id
		}
	}
	packets := make([]*helpers.GossipPacket, 0)
	for i := ps.NextID; i <= maxID; i++ {
		packets = append(packets, idMessages[i].Packet)
	}

	// for id := range (ps.NextID, )
	// idMessagesp[ps.NextID]
	//
	// for id, message := range idMessages {
	// 	if id >= ps.NextID-1 {
	// 		packets = append(packets, message.Packet)
	// 	}
	// }
	return packets
}

func printStatusMessage(extPacket *helpers.ExtendedGossipPacket) {
	message := "STATUS from " + extPacket.SenderAddr.String() + " "
	for _, value := range extPacket.Packet.Status.Want {
		message = message + "peer " + value.Identifier + " nextID " + fmt.Sprint(value.NextID) + " "
	}
	fmt.Println(message[:len(message)-1])
}

func (gossiper *Gossiper) handleConnectionStatus(statusChannel chan *helpers.ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		_, channelCreated := gossiper.statusChannels[extPacket.SenderAddr.String()]
		if channelCreated {
			//gossiper.statusChannels[extPacket.SenderAddr.String()] = make(chan *helpers.ExtendedGossipPacket)
			go func() {
				gossiper.statusChannels[extPacket.SenderAddr.String()] <- extPacket
			}()

		}

		printStatusMessage(extPacket)
		gossiper.peers.addPeer(extPacket.SenderAddr)
		printPeers(gossiper.peers.getPeersAtomic())

		myStatus := gossiper.createStatus()

		toSend := gossiper.getDifferenceStatus(myStatus, extPacket.Packet.Status.Want)
		wanted := gossiper.getDifferenceStatus(extPacket.Packet.Status.Want, myStatus)

		// fmt.Println(len(toSend), len(wanted))
		// if len(toSend) != 0 {
		// 	fmt.Println(toSend[0].Identifier + " " + fmt.Sprint(toSend[0].NextID))
		// }
		// if len(wanted) != 0 {
		// 	fmt.Println(wanted[0].Identifier + " " + fmt.Sprint(wanted[0].NextID))
		// }

		//countSent := 0
		for _, ps := range toSend {
			packets := gossiper.getPacketsFromStatus(ps)
			for _, m := range packets {
				//		countSent = countSent + 1
				fmt.Println("MONGERING with " + extPacket.SenderAddr.String())
				gossiper.sendPacket(m, extPacket.SenderAddr)
			}
		}

		//if len(toSend) == 0 {
		if len(wanted) != 0 {
			go gossiper.sendStatusPacket(extPacket.SenderAddr)
		} else {
			if len(toSend) == 0 {
				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
			}
		}
		//}

		// for _, ps := range extPacket.Packet.Status.Want {
		// 	messages, isPeerKnown := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
		// 	if !isPeerKnown {
		// 		gossiper.sendStatusPacket(extPacket.SenderAddr)
		// 		continue
		// 	} else {
		// 		for k, v := range messages {
		// 			if k.Rumor.ID >= ps.NextID {
		// 				gossiper.sendPacket(k, extPacket.SenderAddr)
		// 			}
		// 		}
		// 		for k, v := range messages {
		// 			if k.Rumor.ID+1 < ps.NextID {
		// 				gossiper.sendStatusPacket(extPacket.SenderAddr)
		// 				continue
		// 			}
		// 		}
		// 	}
		// }
	}
}

func (gossiper *Gossiper) handleConnectionClient(channelClient chan *helpers.ExtendedGossipPacket) {

	for extPacket := range channelClient {

		// modify packet
		if gossiper.simpleMode {

			// print messages
			fmt.Println("CLIENT MESSAGE " + extPacket.Packet.Simple.Contents)
			printPeers(gossiper.peers.getPeersAtomic())

			extPacket.Packet.Simple.OriginalName = gossiper.name
			extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Addr.String()

			// broadcast
			go gossiper.broadcastToPeers(extPacket)
		} else {

			// print messages
			fmt.Println("CLIENT MESSAGE " + extPacket.Packet.Rumor.Text)
			printPeers(gossiper.peers.getPeersAtomic())

			id := gossiper.seqID.getIDAtomic()
			gossiper.seqID.incerementID()
			extPacket.Packet.Rumor.ID = id
			extPacket.Packet.Rumor.Origin = gossiper.name
			gossiper.originPackets.Mutex.Lock()

			_, check := gossiper.originPackets.OriginPacketsMap[gossiper.name]
			if !check {
				gossiper.originPackets.OriginPacketsMap[gossiper.name] = make(map[uint32]*helpers.ExtendedGossipPacket)
			}
			gossiper.originPackets.OriginPacketsMap[gossiper.name][id] = extPacket
			gossiper.originPackets.Mutex.Unlock()
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (peers *MutexPeers) addPeer(peer *net.UDPAddr) {
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	contains := false
	for _, p := range peers.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		peers.Peers = append(peers.Peers, peer)
	}
}

func (mID *MutexSequenceID) incerementID() {
	mID.Mutex.Lock()
	defer mID.Mutex.Unlock()
	mID.ID = mID.ID + 1
}

func (mID *MutexSequenceID) getIDAtomic() uint32 {
	mID.Mutex.Lock()
	defer mID.Mutex.Unlock()
	idCopy := mID.ID
	return idCopy
}

func printPeers(peers []*net.UDPAddr) {
	list := make([]string, 0)
	for _, p := range peers {
		list = append(list, p.String())
	}
	fmt.Println("PEERS " + strings.Join(list, ","))
}

func (peers *MutexPeers) getPeersAtomic() []*net.UDPAddr {
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	peerCopy := make([]*net.UDPAddr, len(peers.Peers))
	copy(peerCopy, peers.Peers)
	return peerCopy
}

func (gossiper *Gossiper) handleConnectionSimple(channelPeers chan *helpers.ExtendedGossipPacket) {
	for extPacket := range channelPeers {

		// print messages
		fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)

		// get receiver address and store it
		gossiper.peers.addPeer(extPacket.SenderAddr)
		printPeers(gossiper.peers.getPeersAtomic())

		// change address in packet
		extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Addr.String()

		// broadcast
		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) getPacket(data *helpers.NetworkData, channels map[string]chan *helpers.ExtendedGossipPacket) {
	for {
		packetFromPeer := &helpers.GossipPacket{}
		packetFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)
		_, addr, err := data.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		if data.Addr.String() == gossiper.clientData.Addr.String() {
			protobuf.Decode(packetBytes, packetFromClient)
			if gossiper.simpleMode {
				simplePacket := &helpers.SimpleMessage{Contents: packetFromClient.Text}
				channels["client"] <- &helpers.ExtendedGossipPacket{Packet: &helpers.GossipPacket{Simple: simplePacket}, SenderAddr: addr}
			} else {
				rumorPacket := &helpers.RumorMessage{Text: packetFromClient.Text}
				channels["client"] <- &helpers.ExtendedGossipPacket{Packet: &helpers.GossipPacket{Rumor: rumorPacket}, SenderAddr: addr}
			}
			//channels["client"] <- &helpers.ExtendedGossipPacket{Packet: *helpers.GossipPacket{}, SenderAddr: addr}
		} else {
			protobuf.Decode(packetBytes, packetFromPeer)
			modeType := helpers.GetTypeMode(packetFromPeer)
			if (modeType == "simple" && gossiper.simpleMode) || (modeType != "simple" && !gossiper.simpleMode) {
				channels[modeType] <- &helpers.ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
			}
		}
	}
}

func (gossiper *Gossiper) broadcastToPeers(packet *helpers.ExtendedGossipPacket) {
	//packetToSend, err := protobuf.Encode(packet.Packet)
	//helpers.ErrorCheck(err)

	// broadcast to peers
	for _, peer := range gossiper.peers.getPeersAtomic() {
		if peer.String() != packet.SenderAddr.String() {
			go gossiper.sendPacket(packet.Packet, peer)
			//gossiper.connGossiper.WriteToUDP(packetToSend, peer)
		}
	}
}

func (gossiper *Gossiper) sendPacket(packet *helpers.GossipPacket, address *net.UDPAddr) {
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	gossiper.gossiperData.Conn.WriteToUDP(packetToSend, address)
}
