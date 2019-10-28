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
	Mutex        sync.Mutex
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

	origin := extPacket.Packet.Rumor.Origin
	address := extPacket.SenderAddr

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

	if extPacket.Packet.Rumor.ID > maxID {

		if extPacket.Packet.Rumor.Text != "" {
			fmt.Println("DSDV " + origin + " " + address.String())
		}

		addressInTable, isPresent := gossiper.routingTable.RoutingTable[origin]

		// add or update entry
		if !isPresent || addressInTable.String() != address.String() {

			gossiper.routingTable.Mutex.Lock()
			gossiper.routingTable.RoutingTable[origin] = address
			gossiper.routingTable.Mutex.Unlock()
		}
	}
}

func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket) {
	if packet.Private.HopLimit > 0 {
		packet.Private.HopLimit = packet.Private.HopLimit - 1

		addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.Private.Destination]
		if isPresent {
			gossiper.sendPacket(packet, addressInTable)
		}
	}
}

func (gossiper *Gossiper) forwardDataRequest(packet *GossipPacket) {
	if packet.DataRequest.HopLimit > 0 {
		packet.DataRequest.HopLimit = packet.DataRequest.HopLimit - 1

		addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataRequest.Destination]
		if isPresent {
			gossiper.sendPacket(packet, addressInTable)
		}
	}
}

func (gossiper *Gossiper) forwardDataReply(packet *GossipPacket) {

	if packet.DataReply.HopLimit > 0 {
		packet.DataReply.HopLimit = packet.DataReply.HopLimit - 1

		addressInTable, isPresent := gossiper.routingTable.RoutingTable[packet.DataReply.Destination]
		if isPresent {
			gossiper.sendPacket(packet, addressInTable)
		}
	}
}

// GetOriginsFromRoutingTable get the list of node ​Origin identifiers known to this node, i.e. for whom a next-hop route is available
func (gossiper *Gossiper) GetOriginsFromRoutingTable() []string {
	origins := make([]string, 0)

	gossiper.routingTable.Mutex.Lock()
	defer gossiper.routingTable.Mutex.Unlock()

	for o := range gossiper.routingTable.RoutingTable {
		origins = append(origins, o)
	}

	return origins
}
