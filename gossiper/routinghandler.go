package gossiper

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// RoutingHandler struct
type RoutingHandler struct {
	RoutingTable map[string]*net.UDPAddr
	OriginLastID *VectorClock
	Mutex        sync.RWMutex
}


// NewRoutingHandler create new routing handler
func NewRoutingHandler() *RoutingHandler {
	return &RoutingHandler{
		RoutingTable: make(map[string]*net.UDPAddr),
		OriginLastID: &VectorClock{Entries: make(map[string]uint32)},
	}
}

// StartRouteRumormongering with the specified timer
func (gossiper *Gossiper) StartRouteRumormongering(routeTimer uint) {

	if routeTimer > 0 {

		// create new rumor message
		id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
		atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))
		rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		gossiper.storeMessage(extPacket.Packet, gossiper.name, id)
		
		// broadcast it initially in order to start well
		gossiper.broadcastToPeers(extPacket)

		// start timer
		timer := time.NewTicker(time.Duration(routeTimer) * time.Second)
		for {
			select {
			// rumor monger rumor at each timeout
			case <-timer.C:
				gossiper.mongerRouteRumor()
			}
		}
	}
}

// rumor monger route rumor
func (gossiper *Gossiper) mongerRouteRumor() {

	// create new rumor message
	id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
	atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))
	rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
	gossiper.storeMessage(extPacket.Packet, gossiper.name, id)

	// start rumormongering the message
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)
}

// update routing table based on packet data
func (gossiper *Gossiper) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

	gossiper.routingHandler.Mutex.Lock()
	defer gossiper.routingHandler.Mutex.Unlock()

	// if new packet with higher id, update table
	if origin != gossiper.name && gossiper.checkAndUpdateLastOriginID(origin, idPacket) {

		if textPacket != "" {
			if hw2 {
				fmt.Println("DSDV " + origin + " " + address.String())
			}
		}

		// update
		gossiper.routingHandler.RoutingTable[origin] = address

		if debug {
			fmt.Println("Routing table updated")
		}
	}
}

// check if packet is new (has higher id) from that source
func (gossiper *Gossiper) checkAndUpdateLastOriginID(origin string, id uint32) bool {
	isNew := false

	originMaxID, loaded := gossiper.routingHandler.OriginLastID.Entries[origin]
	if !loaded {
		gossiper.routingHandler.OriginLastID.Entries[origin] = id
		isNew = true
	} else {
		if id > originMaxID {
			gossiper.routingHandler.OriginLastID.Entries[origin] = id
			isNew = true
		}
	}

	return isNew
}

// forward private message
func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket, hopLimit *uint32, destination string) {

	// check if hop limit is valid, decrement it if ok
	if *hopLimit > 0 {
		*hopLimit = *hopLimit - 1

		// get routing table entry for the origin
		gossiper.routingHandler.Mutex.RLock()
		addressInTable, isPresent := gossiper.routingHandler.RoutingTable[destination]
		gossiper.routingHandler.Mutex.RUnlock()

		// send packet if address is present
		if isPresent {
			gossiper.sendPacket(packet, addressInTable)
		}
	}
}

// GetOriginsAtomic in concurrent environment
func (gossiper *Gossiper) GetOriginsAtomic() []string {
	gossiper.routingHandler.Mutex.RLock()
	defer gossiper.routingHandler.Mutex.RUnlock()

	origins := make([]string, len(gossiper.routingHandler.OriginLastID.Entries))
	i := 0
	for k := range gossiper.routingHandler.OriginLastID.Entries {
    	origins[i] = k
    	i++
	}
	return origins
}
