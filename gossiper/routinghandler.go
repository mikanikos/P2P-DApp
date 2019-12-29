package gossiper

import (
	"fmt"
	"net"
	"sync"
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
func (gossiper *Gossiper) startRouteRumormongering() {

	if routeRumorTimeout > 0 {

		if debug {
			fmt.Println("ok herere")
		}

		// create new rumor message
		extPacket := gossiper.createRumorMessage("")

		// broadcast it initially in order to start well
		go gossiper.broadcastToPeers(extPacket)

		// start timer
		timer := time.NewTicker(time.Duration(routeRumorTimeout) * time.Second)
		for {
			select {
			// rumor monger rumor at each timeout
			case <-timer.C:
				// create new rumor message
				extPacket := gossiper.createRumorMessage("")

				// start rumormongering the message
				go gossiper.startRumorMongering(extPacket, gossiper.name, extPacket.Packet.Rumor.ID)
			}
		}
	}
}

// update routing table based on packet data
func (gossiper *Gossiper) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

	gossiper.routingHandler.Mutex.Lock()
	defer gossiper.routingHandler.Mutex.Unlock()

	// if new packet with higher id, update table
	if gossiper.checkAndUpdateLastOriginID(origin, idPacket) {

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
		} else {
			// // broadcast, hoping some peers have a routing entry
			// peers := gossiper.GetPeersAtomic()
			// for _, peer := range peers {
			// 	gossiper.sendPacket(packet, peer)
			// }
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
