package gossiper

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// MutexRoutingTable struct
// type MutexRoutingTable struct {
// 	RoutingTable map[string]*net.UDPAddr
// 	Mutex        sync.Mutex
// }

// RoutingTable struct
type RoutingTable struct {
	Table   sync.Map
	Origins chan string
}

func (gossiper *Gossiper) startRouteRumormongering() {

	if gossiper.routeTimer > 0 {

		// startup route rumor
		gossiper.mongerRouteRumor()

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

	idMessages, isMessageKnown := gossiper.originPackets.OriginPacketsMap.Load(origin)

	var maxID uint32 = 0
	if isMessageKnown {

		idMessages.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
			id := key.(uint32)
			if id > maxID {
				maxID = id
			}
			return true
		})
	}

	if idPacket > maxID || extPacket.Packet.Private != nil {

		if textPacket != "" {
			fmt.Println("DSDV " + origin + " " + address.String())
		}

		// gossiper.routingTable.Mutex.Lock()
		// defer gossiper.routingTable.Mutex.Unlock()

		//_, isPresent := gossiper.routingTable.RoutingTable[origin]

		_, loaded := gossiper.routingTable.Table.LoadOrStore(origin, address)

		if !loaded {
			go func(o string) {
				gossiper.routingTable.Origins <- o
			}(origin)
		}

		// // add or update entry
		// if !isPresent { //|| addressInTable.String() != address.String() {
		// 	gossiper.routingTable.RoutingTable[origin] = address
		// }
	}
}

func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket) {
	if packet.Private.HopLimit > 0 {
		packet.Private.HopLimit = packet.Private.HopLimit - 1

		// gossiper.routingTable.Mutex.Lock()
		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.Private.Destination]
		// gossiper.routingTable.Mutex.Unlock()

		value, isPresent := gossiper.routingTable.Table.Load(packet.Private.Destination)

		if isPresent {
			addressInTable := value.(*net.UDPAddr)
			gossiper.sendPacket(packet, addressInTable)
		} else {
			peersCopy := gossiper.GetPeersAtomic()
			if len(peersCopy) != 0 {
				randomPeer := gossiper.getRandomPeer(peersCopy)
				gossiper.sendPacket(packet, randomPeer)
			}
		}
	}
}

func (gossiper *Gossiper) forwardDataRequest(packet *GossipPacket) {
	if packet.DataRequest.HopLimit > 0 {
		packet.DataRequest.HopLimit = packet.DataRequest.HopLimit - 1

		// gossiper.routingTable.Mutex.Lock()
		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataRequest.Destination]
		// gossiper.routingTable.Mutex.Unlock()

		value, isPresent := gossiper.routingTable.Table.Load(packet.DataRequest.Destination)

		if isPresent {
			addressInTable := value.(*net.UDPAddr)
			gossiper.sendPacket(packet, addressInTable)
		} else {
			peersCopy := gossiper.GetPeersAtomic()
			if len(peersCopy) != 0 {
				randomPeer := gossiper.getRandomPeer(peersCopy)
				gossiper.sendPacket(packet, randomPeer)
			}
		}
	}
}

func (gossiper *Gossiper) forwardDataReply(packet *GossipPacket) {

	if packet.DataReply.HopLimit > 0 {
		packet.DataReply.HopLimit = packet.DataReply.HopLimit - 1

		// gossiper.routingTable.Mutex.Lock()
		// addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataReply.Destination]
		// gossiper.routingTable.Mutex.Unlock()

		value, isPresent := gossiper.routingTable.Table.Load(packet.DataReply.Destination)

		if isPresent {
			addressInTable := value.(*net.UDPAddr)
			gossiper.sendPacket(packet, addressInTable)
		} else {
			peersCopy := gossiper.GetPeersAtomic()
			if len(peersCopy) != 0 {
				randomPeer := gossiper.getRandomPeer(peersCopy)
				gossiper.sendPacket(packet, randomPeer)
			}
		}
	}
}

// GetOriginsFromRoutingTable get the list of node ​Origin identifiers known to this node, i.e. for whom a next-hop route is available
func (gossiper *Gossiper) GetOriginsFromRoutingTable() []string {
	//origins := make([]string, 0)

	// gossiper.routingTable.Mutex.Lock()
	// defer gossiper.routingTable.Mutex.Unlock()

	// for o := range gossiper.routingTable.Origins {
	// 	origins = append(origins, o)
	// }

	bufferLength := len(gossiper.routingTable.Origins)

	origins := make([]string, bufferLength)
	for i := 0; i < bufferLength; i++ {
		o := <-gossiper.routingTable.Origins
		origins[i] = o
	}

	return origins
}
