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

// StartRouteRumormongering with the specified timer
func (gossiper *Gossiper) StartRouteRumormongering(routeTimer uint) {

	if routeTimer > 0 {

		// DIFFERENT METHOD: BROADCAST START-UP ROUTE RUMOR INSTEAD OF MONGERING IT

		// id := atomic.LoadUint32(&gossiper.seqID)
		// atomic.AddUint32(&gossiper.seqID, uint32(1))
		// rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		// extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		// gossiper.addMessage(extPacket)
		// gossiper.broadcastToPeers(extPacket)

		gossiper.mongerRouteRumor()

		timer := time.NewTicker(time.Duration(routeTimer) * time.Second)
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
	gossiper.storeRumorMessage(extPacket.Packet.Rumor)

	go gossiper.startRumorMongering(extPacket)
}

func (gossiper *Gossiper) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

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

func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket, hopLimit *uint32, destination string) {

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
