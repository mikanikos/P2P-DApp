package gossiper

import (
	"sync"
	"sync/atomic"
)

// GossipHandler struct
type GossipHandler struct {
	// seq id for gossips
	seqID uint32
	// store gossip messages sent and received
	messageStorage sync.Map
	// my current status (vector clock)
	myStatus *VectorClock
	// status channels used to process status sync with peers in paraller
	statusChannels sync.Map
	// channels used to communicate the arrival of status packets for active rumormongering goroutines
	mongeringChannels sync.Map
	// send rumors to gui
	latestRumors chan *RumorMessage
}

// NewGossipHandler create new gossip handler
func NewGossipHandler() *GossipHandler {
	return &GossipHandler{
		seqID:             1,
		messageStorage:    sync.Map{},
		myStatus:          &VectorClock{Entries: make(map[string]uint32)},
		statusChannels:    sync.Map{},
		mongeringChannels: sync.Map{},
		latestRumors:      make(chan *RumorMessage, latestMessagesBuffer),
	}
}

func (gossiper *Gossiper) handleGossipMessage(extPacket *ExtendedGossipPacket, origin string, id uint32) {

	gossiper.printPeerMessage(extPacket, gossiper.GetPeers())

	packetType := getTypeFromGossip(extPacket.Packet)

	isMessageKnown := true

	if origin != gossiper.name {

		// update routing table
		textMessage := ""
		if packetType == "rumor" {
			textMessage = extPacket.Packet.Rumor.Text
		}
		gossiper.routingHandler.updateRoutingTable(origin, textMessage, id, extPacket.SenderAddr)

		// store message
		isMessageKnown = gossiper.gossipHandler.storeMessage(extPacket.Packet, origin, id)
	}

	// send status
	statusToSend := gossiper.gossipHandler.myStatus.createMyStatusPacket()
	gossiper.connectionHandler.sendPacket(&GossipPacket{Status: statusToSend}, extPacket.SenderAddr)

	if !isMessageKnown {

		if packetType == "rumor" {
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
	id := atomic.LoadUint32(&gossiper.gossipHandler.seqID)
	atomic.AddUint32(&gossiper.gossipHandler.seqID, uint32(1))
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

	value, _ := gossipHandler.messageStorage.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, packet)

	// update status after storing message
	gossipHandler.updateStatus(origin, id, mapValue)

	return loaded
}
