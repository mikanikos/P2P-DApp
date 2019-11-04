package gossiper

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// MutexRoutingTable struct
type MutexRoutingTable struct {
	RoutingTable map[string]*net.UDPAddr
	Mutex        sync.RWMutex
}

// RoutingTable struct
// type RoutingTable struct {
// 	Table sync.Map
// }

func (gossiper *Gossiper) startRouteRumormongering() {

	if gossiper.routeTimer > 0 {

		// startup route rumor
		id := atomic.LoadUint32(&gossiper.seqID)
		atomic.AddUint32(&gossiper.seqID, uint32(1))
		rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		gossiper.addMessage(extPacket)

		gossiper.broadcastToPeers(extPacket)

		timer := time.NewTicker(time.Duration(gossiper.routeTimer) * time.Second)
		for {
			select {
			case <-timer.C:
				gossiper.mongerRouteRumor()
			}
		}
	}
}

func (gossiper *Gossiper) mongerRouteRumor() {
	peersCopy := gossiper.GetPeersAtomic()
	if len(peersCopy) != 0 {

		id := atomic.LoadUint32(&gossiper.seqID)
		atomic.AddUint32(&gossiper.seqID, uint32(1))
		rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		gossiper.addMessage(extPacket)

		randomPeer := gossiper.getRandomPeer(peersCopy)
		gossiper.sendPacket(extPacket.Packet, randomPeer)
	}
}

func (gossiper *Gossiper) updateRoutingTable(extPacket *ExtendedGossipPacket) {

	var origin string
	var textPacket string
	var idPacket uint32
	address := extPacket.SenderAddr

	switch typePacket := getTypeFromGossip(extPacket.Packet); typePacket {

	case "rumor":
		origin = extPacket.Packet.Rumor.Origin
		idPacket = extPacket.Packet.Rumor.ID
		textPacket = extPacket.Packet.Rumor.Text

	case "private":
		origin = extPacket.Packet.Private.Origin
		idPacket = extPacket.Packet.Private.ID
		textPacket = extPacket.Packet.Private.Text
	}

	// idMessages, isMessageKnown := gossiper.originPackets.OriginPacketsMap.Load(origin)

	// var maxID uint32 = 0
	// if isMessageKnown {

	// 	idMessages.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
	// 		id := key.(uint32)
	// 		if id > maxID {
	// 			maxID = id
	// 		}
	// 		return true
	// 	})
	// }

	check := gossiper.checkAndUpdateLastOriginID(origin, idPacket)

	if check || extPacket.Packet.Private != nil {

		if textPacket != "" {
			if hw2 {
				fmt.Println("DSDV " + origin + " " + address.String())
			}
		}

		gossiper.routingTable.Mutex.Lock()

		_, loaded := gossiper.routingTable.RoutingTable[origin]
		gossiper.routingTable.RoutingTable[origin] = address

		gossiper.routingTable.Mutex.Unlock()

		if !loaded {
			gossiper.addOrigin(origin)

		}

		if debug {
			fmt.Println("Routing table updated")
		}
	}
}

func (gossiper *Gossiper) checkAndUpdateLastOriginID(origin string, id uint32) bool {
	gossiper.originLastID.Mutex.Lock()
	defer gossiper.originLastID.Mutex.Unlock()

	isNew := false

	originMaxID, loaded := gossiper.originLastID.Status[origin]
	if !loaded {
		gossiper.originLastID.Status[origin] = id
		isNew = true
	} else {
		if id > originMaxID {
			gossiper.originLastID.Status[origin] = id
			isNew = true
		}
	}

	return isNew
}

func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket) {

	var hopLimit *uint32
	var destination string

	switch packetType := getTypeFromGossip(packet); packetType {

	case "private":
		hopLimit = &packet.Private.HopLimit
		destination = packet.Private.Destination

	case "request":
		hopLimit = &packet.DataRequest.HopLimit
		destination = packet.DataRequest.Destination

	case "reply":
		hopLimit = &packet.DataReply.HopLimit
		destination = packet.DataReply.Destination
	}

	if *hopLimit > 0 {
		*hopLimit = *hopLimit - 1

		// gossiper.routingTable.Mutex.Lock()
		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.Private.Destination]
		// gossiper.routingTable.Mutex.Unlock()

		gossiper.routingTable.Mutex.RLock()
		addressInTable, isPresent := gossiper.routingTable.RoutingTable[destination]
		gossiper.routingTable.Mutex.RUnlock()

		if isPresent {
			//addressInTable := value.(*net.UDPAddr)
			gossiper.sendPacket(packet, addressInTable)
		} else {
			// IF NO ENTRIES IN ROUTING TABLE, SHOULD I SEND TO A RANDOM PEER (WHO MIGHT HAVE THE ENTRY) IN ORDER TO NOT LOSE THE PACKET?

			// peersCopy := gossiper.GetPeersAtomic()
			// if len(peersCopy) != 0 {
			// 	randomPeer := gossiper.getRandomPeer(peersCopy)
			// 	gossiper.sendPacket(packet, randomPeer)
			// }
		}
	}
}

// func (gossiper *Gossiper) forwardDataRequest(packet *GossipPacket) {
// 	if packet.DataRequest.HopLimit > 0 {
// 		packet.DataRequest.HopLimit = packet.DataRequest.HopLimit - 1

// 		// gossiper.routingTable.Mutex.Lock()
// 		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataRequest.Destination]
// 		// gossiper.routingTable.Mutex.Unlock()

// 		value, isPresent := gossiper.routingTable.Table.Load(packet.DataRequest.Destination)

// 		if isPresent {
// 			addressInTable := value.(*net.UDPAddr)
// 			gossiper.sendPacket(packet, addressInTable)
// 		} else {
// 			peersCopy := gossiper.GetPeersAtomic()
// 			if len(peersCopy) != 0 {
// 				randomPeer := gossiper.getRandomPeer(peersCopy)
// 				gossiper.sendPacket(packet, randomPeer)
// 			}
// 		}
// 	}
// }

// func (gossiper *Gossiper) forwardDataReply(packet *GossipPacket) {

// 	if packet.DataReply.HopLimit > 0 {
// 		packet.DataReply.HopLimit = packet.DataReply.HopLimit - 1

// 		// gossiper.routingTable.Mutex.Lock()
// 		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataReply.Destination]
// 		// gossiper.routingTable.Mutex.Unlock()

// 		value, isPresent := gossiper.routingTable.Table.Load(packet.DataReply.Destination)

// 		if isPresent {
// 			addressInTable := value.(*net.UDPAddr)
// 			gossiper.sendPacket(packet, addressInTable)
// 		} else {
// 			peersCopy := gossiper.GetPeersAtomic()
// 			if len(peersCopy) != 0 {
// 				randomPeer := gossiper.getRandomPeer(peersCopy)
// 				gossiper.sendPacket(packet, randomPeer)
// 			}
// 		}
// 	}
// }

// GetOriginsFromRoutingTable get the list of node ​Origin identifiers known to this node, i.e. for whom a next-hop route is available
// func (gossiper *Gossiper) GetOriginsFromRoutingTable() []string {
// 	//origins := make([]string, 0)

// 	// gossiper.routingTable.Mutex.Lock()
// 	// defer gossiper.routingTable.Mutex.Unlock()

// 	// for o := range gossiper.routingTable.Origins {
// 	// 	origins = append(origins, o)
// 	// }

// 	// bufferLength := len(gossiper.routingTable.Origins)

// 	// origins := make([]string, bufferLength)
// 	// for i := 0; i < bufferLength; i++ {
// 	// 	o := <-gossiper.routingTable.Origins
// 	// 	origins[i] = o
// 	// }

// 	// return origins
// }
