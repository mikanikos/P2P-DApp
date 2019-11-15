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

func (gossiper *Gossiper) startRouteRumormongering() {

	if gossiper.routeTimer > 0 {

		// DIFFERENT METHOD: BROADCAST START-UP ROUTE RUMOR INSTEAD OF MONGERING IT

		// id := atomic.LoadUint32(&gossiper.seqID)
		// atomic.AddUint32(&gossiper.seqID, uint32(1))
		// rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		// extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		// gossiper.addMessage(extPacket)
		// gossiper.broadcastToPeers(extPacket)

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

	id := atomic.LoadUint32(&gossiper.seqID)
	atomic.AddUint32(&gossiper.seqID, uint32(1))
	rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
	gossiper.addMessage(extPacket)

	go gossiper.startRumorMongering(extPacket)
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

	if gossiper.checkAndUpdateLastOriginID(origin, idPacket) {

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

	case "dataRequest":
		hopLimit = &packet.DataRequest.HopLimit
		destination = packet.DataRequest.Destination

	case "dataReply":
		hopLimit = &packet.DataReply.HopLimit
		destination = packet.DataReply.Destination

	case "searchReply":
		hopLimit = &packet.SearchReply.HopLimit
		destination = packet.SearchReply.Destination
	}

	if *hopLimit > 0 {
		*hopLimit = *hopLimit - 1

		gossiper.routingTable.Mutex.RLock()
		addressInTable, isPresent := gossiper.routingTable.RoutingTable[destination]
		gossiper.routingTable.Mutex.RUnlock()

		if isPresent {
			gossiper.sendPacket(packet, addressInTable)
		}
	}
}
