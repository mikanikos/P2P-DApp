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

		// create new rumor message
		extPacket := gossiper.createRumorMessage("")

		// broadcast it initially in order to start well
		go gossiper.connectionHandler.broadcastToPeers(extPacket, gossiper.GetPeers())

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
func (routingHandler *RoutingHandler) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

	routingHandler.Mutex.Lock()
	defer routingHandler.Mutex.Unlock()

	// if new packet with higher id, update table
	if routingHandler.updateLastOriginID(origin, idPacket) {

		if textPacket != "" {
			if hw2 {
				fmt.Println("DSDV " + origin + " " + address.String())
			}
		}

		// update
		routingHandler.RoutingTable[origin] = address

		if debug {
			fmt.Println("Routing table updated")
		}
	}
}

// check if packet is new (has higher id) from that source, in that case it updates the table
func (routingHandler *RoutingHandler) updateLastOriginID(origin string, id uint32) bool {
	isNew := false

	originMaxID, loaded := routingHandler.OriginLastID.Entries[origin]
	if !loaded {
		routingHandler.OriginLastID.Entries[origin] = id
		isNew = true
	} else {
		if id > originMaxID {
			routingHandler.OriginLastID.Entries[origin] = id
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
			gossiper.connectionHandler.sendPacket(packet, addressInTable)
		}
	}
}

// GetOrigins in concurrent environment
func (gossiper *Gossiper) GetOrigins() []string {
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
