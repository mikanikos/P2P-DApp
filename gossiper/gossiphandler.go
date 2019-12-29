package gossiper

import (
	"sync"
	"sync/atomic"
)

// GossipHandler struct
type GossipHandler struct {
	SeqID             uint32
	MessageStorage    sync.Map
	MyStatus          VectorClock
	StatusChannels    sync.Map
	MongeringChannels sync.Map
	latestRumors      chan *RumorMessage
}

// NewGossipHandler create new gossip handler
func NewGossipHandler() *GossipHandler {
	return &GossipHandler{
		SeqID:             1,
		MessageStorage:    sync.Map{},
		MyStatus:          VectorClock{Entries: make(map[string]uint32)},
		StatusChannels:    sync.Map{},
		MongeringChannels: sync.Map{},
		latestRumors:      make(chan *RumorMessage, latestMessagesBuffer),
	}
}

// VectorClock struct
type VectorClock struct {
	Entries map[string]uint32
	Mutex   sync.RWMutex
}

func (gossiper *Gossiper) handleGossipMessage(extPacket *ExtendedGossipPacket, origin string, id uint32) {

	gossiper.printPeerMessage(extPacket, gossiper.GetPeers())

	isMessageKnown := true

	if origin != gossiper.name {

		// update routing table
		textMessage := ""
		if getTypeFromGossip(extPacket.Packet) == "rumor" {
			textMessage = extPacket.Packet.Rumor.Text
		}
		gossiper.routingHandler.updateRoutingTable(origin, textMessage, id, extPacket.SenderAddr)

		// store message
		isMessageKnown = gossiper.gossipHandler.storeMessage(extPacket.Packet, origin, id)
	}

	// send status
	statusToSend := gossiper.gossipHandler.MyStatus.createMyStatusPacket()
	gossiper.connectionHandler.sendPacket(&GossipPacket{Status: statusToSend}, extPacket.SenderAddr)

	if !isMessageKnown {

		if getTypeFromGossip(extPacket.Packet) == "rumor" {
			if extPacket.Packet.Rumor.Text != "" {
				go func(r *RumorMessage) {
					gossiper.gossipHandler.latestRumors <- r
				}(extPacket.Packet.Rumor)
			}
		}

		// start rumor monger
		go gossiper.startRumorMongering(extPacket, origin, id)
	}
}

// create new rumor message
func (gossiper *Gossiper) createRumorMessage(text string) *ExtendedGossipPacket {
	id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
	atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))
	rumorPacket := &RumorMessage{Origin: gossiper.name, ID: id, Text: text}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.connectionHandler.gossiperData.Address}
	gossiper.gossipHandler.storeMessage(extPacket.Packet, gossiper.name, id)

	if text != "" {
		go func(r *RumorMessage) {
			gossiper.gossipHandler.latestRumors <- r
		}(extPacket.Packet.Rumor)
	}

	return extPacket
}

// store gossip message based on origin and seid
func (gossipHandler *GossipHandler) storeMessage(packet *GossipPacket, origin string, id uint32) bool {

	value, _ := gossipHandler.MessageStorage.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, packet)

	// update status after storing message
	gossipHandler.updateStatus(origin, id, mapValue)

	return loaded
}
