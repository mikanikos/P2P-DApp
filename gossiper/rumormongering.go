package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

var rumorTimeout = 10

// RumorMessage struct
type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

// StatusPacket struct
type StatusPacket struct {
	Want []PeerStatus
}

// PeerStatus struct
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

func (gossiper *Gossiper) startRumorMongering(extPacket *ExtendedGossipPacket) {
	coin := 1

	peersWithRumor := make([]*net.UDPAddr, 0)
	peersWithRumor = append(peersWithRumor, extPacket.SenderAddr)

	for coin == 1 {
		peers := gossiper.GetPeersAtomic()
		availablePeers := helpers.DifferenceString(peers, peersWithRumor)
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

func (gossiper *Gossiper) sendRumorWithTimeout(packet *GossipPacket, peer *net.UDPAddr) bool {
	gossiper.statusChannels[peer.String()] = make(chan *ExtendedGossipPacket)

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

func (gossiper *Gossiper) createStatus() []PeerStatus {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	myStatus := make([]PeerStatus, 0)
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
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: maxID + 1})
		//}
	}
	return myStatus
}

func (gossiper *Gossiper) getDifferenceStatus(myStatus, otherStatus []PeerStatus) []PeerStatus {
	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}
	difference := make([]PeerStatus, 0)
	for _, elem := range myStatus {
		//message, _ := gossiper.originPackets.OriginPacketsMap[elem.Identifier][elem.NextID-1]
		//if elem.NextID == 1 || message.SenderAddr.String() != otherAddr {
		id, isOriginKnown := originIDMap[elem.Identifier]
		if !isOriginKnown {
			difference = append(difference, PeerStatus{Identifier: elem.Identifier, NextID: 1})
		} else if elem.NextID > id {
			difference = append(difference, PeerStatus{Identifier: elem.Identifier, NextID: id})
		}
		//}
	}
	return difference
}

func (gossiper *Gossiper) getPacketsFromStatus(ps PeerStatus) []*GossipPacket {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	idMessages, _ := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
	maxID := ps.NextID
	for id := range idMessages {
		if id > maxID {
			maxID = id
		}
	}
	packets := make([]*GossipPacket, 0)
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

// func (gossiper *Gossiper) handleConnectionStatus(statusChannel chan *helpers.ExtendedGossipPacket) {
// 	for extPacket := range statusChannel {

// 		_, channelCreated := gossiper.statusChannels[extPacket.SenderAddr.String()]
// 		if channelCreated {
// 			//gossiper.statusChannels[extPacket.SenderAddr.String()] = make(chan *helpers.ExtendedGossipPacket)
// 			go func() {
// 				gossiper.statusChannels[extPacket.SenderAddr.String()] <- extPacket
// 			}()

// 		}

// 		printStatusMessage(extPacket)
// 		gossiper.peers.AddPeer(extPacket.SenderAddr)
// 		printPeers(gossiper.peers.GetPeersAtomic())

// 		myStatus := gossiper.createStatus()

// 		toSend := gossiper.getDifferenceStatus(myStatus, extPacket.Packet.Status.Want)
// 		wanted := gossiper.getDifferenceStatus(extPacket.Packet.Status.Want, myStatus)

// 		// fmt.Println(len(toSend), len(wanted))
// 		// if len(toSend) != 0 {
// 		// 	fmt.Println(toSend[0].Identifier + " " + fmt.Sprint(toSend[0].NextID))
// 		// }
// 		// if len(wanted) != 0 {
// 		// 	fmt.Println(wanted[0].Identifier + " " + fmt.Sprint(wanted[0].NextID))
// 		// }

// 		//countSent := 0
// 		for _, ps := range toSend {
// 			packets := gossiper.getPacketsFromStatus(ps)
// 			for _, m := range packets {
// 				//		countSent = countSent + 1
// 				fmt.Println("MONGERING with " + extPacket.SenderAddr.String())
// 				gossiper.sendPacket(m, extPacket.SenderAddr)
// 			}
// 		}

// 		//if len(toSend) == 0 {
// 		if len(wanted) != 0 {
// 			go gossiper.sendStatusPacket(extPacket.SenderAddr)
// 		} else {
// 			if len(toSend) == 0 {
// 				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
// 			}
// 		}
// 		//}

// 		// for _, ps := range extPacket.Packet.Status.Want {
// 		// 	messages, isPeerKnown := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
// 		// 	if !isPeerKnown {
// 		// 		gossiper.sendStatusPacket(extPacket.SenderAddr)
// 		// 		continue
// 		// 	} else {
// 		// 		for k, v := range messages {
// 		// 			if k.Rumor.ID >= ps.NextID {
// 		// 				gossiper.sendPacket(k, extPacket.SenderAddr)
// 		// 			}
// 		// 		}
// 		// 		for k, v := range messages {
// 		// 			if k.Rumor.ID+1 < ps.NextID {
// 		// 				gossiper.sendStatusPacket(extPacket.SenderAddr)
// 		// 				continue
// 		// 			}
// 		// 		}
// 		// 	}
// 		// }
// 	}
// }
