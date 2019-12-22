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

		id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
		atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))
		rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
		gossiper.storeMessage(extPacket.Packet, gossiper.name, id)
		gossiper.broadcastToPeers(extPacket)

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

	id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
	atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))
	rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: ""}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Addr}
	gossiper.storeMessage(extPacket.Packet, gossiper.name, id)

	go gossiper.startRumorMongering(extPacket, gossiper.name, id)
}

func (gossiper *Gossiper) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

	gossiper.routingHandler.Mutex.Lock()
	defer gossiper.routingHandler.Mutex.Unlock()

	if origin != gossiper.name && gossiper.checkAndUpdateLastOriginID(origin, idPacket) {

		if textPacket != "" {
			if hw2 {
				fmt.Println("DSDV " + origin + " " + address.String())
			}
		}

		gossiper.routingHandler.RoutingTable[origin] = address

		if debug {
			fmt.Println("Routing table updated")
		}
	}
}

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

func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket, hopLimit *uint32, destination string) {

	if *hopLimit > 0 {
		*hopLimit = *hopLimit - 1

		gossiper.routingHandler.Mutex.RLock()
		addressInTable, isPresent := gossiper.routingHandler.RoutingTable[destination]
		gossiper.routingHandler.Mutex.RUnlock()

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
