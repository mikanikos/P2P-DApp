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
		gossiper.addEntryRoutingTable(origin, address)
	}
}

func (gossiper *Gossiper) addEntryRoutingTable(origin string, address *net.UDPAddr) {
	addressInTable, isPresent := gossiper.routingTable.RoutingTable[origin]

	if !isPresent || addressInTable.String() != address.String() {

		gossiper.routingTable.Mutex.Lock()
		gossiper.routingTable.RoutingTable[origin] = address
		gossiper.routingTable.Mutex.Unlock()
	}
}

func (gossiper *Gossiper) forwardPrivateMessage(extPacket *ExtendedGossipPacket) {
	//fmt.Println(gossiper.routingTable.RoutingTable)
	addressInTable, isPresent := gossiper.routingTable.RoutingTable[extPacket.Packet.Private.Destination]
	if isPresent {
		gossiper.sendPacket(extPacket.Packet, addressInTable)
	}
}

func (gossiper *Gossiper) processPrivateMessage(extPacket *ExtendedGossipPacket) {
	if extPacket.Packet.Private.HopLimit > 0 {
		extPacket.Packet.Private.HopLimit = extPacket.Packet.Private.HopLimit - 1
		go gossiper.forwardPrivateMessage(extPacket)
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
