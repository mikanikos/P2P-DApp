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
}

// NewGossipHandler create new gossip handler
func NewGossipHandler() *GossipHandler {
	return &GossipHandler{
		SeqID:             1,
		MessageStorage:    sync.Map{},
		MyStatus:          VectorClock{Entries: make(map[string]uint32)},
		StatusChannels:    sync.Map{},
		MongeringChannels: sync.Map{},
	}
}

// VectorClock struct
type VectorClock struct {
	Entries map[string]uint32
	Mutex   sync.RWMutex
}

func (gossiper *Gossiper) handleGossipMessage(extPacket *ExtendedGossipPacket, origin string, id uint32) {

	gossiper.printPeerMessage(extPacket, gossiper.GetPeersAtomic())

	isMessageKnown := true

	if origin != gossiper.name {

		// update routing table
		textMessage := ""
		if getTypeFromGossip(extPacket.Packet) == "rumor" {
			textMessage = extPacket.Packet.Rumor.Text
		} 
		gossiper.updateRoutingTable(origin, textMessage, id, extPacket.SenderAddr)

		// store message
		isMessageKnown = gossiper.storeMessage(extPacket.Packet, origin, id)
	}

	// send status
	statusToSend := getStatusToSend(&gossiper.gossipHandler.MyStatus)
	gossiper.sendPacket(&GossipPacket{Status: statusToSend}, extPacket.SenderAddr)

	if !isMessageKnown {

		if getTypeFromGossip(extPacket.Packet) == "rumor" {
			if extPacket.Packet.Rumor.Text != "" {
				go func(r *RumorMessage) {
					gossiper.uiHandler.latestRumors <- r
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
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: gossiper.gossiperData.Address}
	gossiper.storeMessage(extPacket.Packet, gossiper.name, id)

	if text != "" {
		go func(r *RumorMessage) {
			gossiper.uiHandler.latestRumors <- r
		}(extPacket.Packet.Rumor)
	}

	return extPacket
}

// store gossip message based on origin and seid
func (gossiper *Gossiper) storeMessage(packet *GossipPacket, origin string, id uint32) bool {

	value, _ := gossiper.gossipHandler.MessageStorage.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, packet)

	// update status after storing message
	gossiper.updateStatus(origin, id, mapValue)

	return loaded
}

// update my vector clock based on current information of mapvalue
func (gossiper *Gossiper) updateStatus(origin string, id uint32, mapValue *sync.Map) {

	gossiper.gossipHandler.MyStatus.Mutex.Lock()
	defer gossiper.gossipHandler.MyStatus.Mutex.Unlock()

	value, peerExists := gossiper.gossipHandler.MyStatus.Entries[origin]
	maxID := uint32(1)
	if peerExists {
		maxID = uint32(value)
	}

	// increment up to the maximum consecutive known message, i.e. least unknown message
	if maxID <= id {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossiper.gossipHandler.MyStatus.Entries[origin] = maxID
		}
	}
}

// get gossip packet from peer status
func (gossiper *Gossiper) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossiper.gossipHandler.MessageStorage.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return message.(*GossipPacket)
}

// create status packet from current vector clock
func getStatusToSend(status *VectorClock) *StatusPacket {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range status.Entries {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return statusPacket
}

// compare peer status and current status and get an entry the other needs
func getPeerStatusForPeer(otherStatus []PeerStatus, status *VectorClock) *PeerStatus {

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for origin, nextID := range status.Entries {
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown {
			return &PeerStatus{Identifier: origin, NextID: 1}
		} else if nextID > id {
			return &PeerStatus{Identifier: origin, NextID: id}
		}
	}
	return nil
}

// check if I need anything from the other peer status
func isPeerStatusNeeded(otherStatus []PeerStatus, status *VectorClock) bool {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := status.Entries[elem.Identifier]
		if !isOriginKnown {
			return true
		} else if elem.NextID > id {
			return true
		}
	}
	return false
}

// MessageUniqueID struct
type MessageUniqueID struct {
	Origin string
	ID     uint32
}

// get listener for incoming status
func (gossiper *Gossiper) getListenerForStatus(origin string, id uint32, peer string) (chan bool, bool) {
	msgChan, _ := gossiper.gossipHandler.MongeringChannels.LoadOrStore(peer, &sync.Map{})
	channel, loaded := msgChan.(*sync.Map).LoadOrStore(MessageUniqueID{Origin: origin, ID: id}, make(chan bool, maxChannelSize))
	return channel.(chan bool), loaded
}

// delete listener for incoming status
func (gossiper *Gossiper) deleteListenerForStatus(origin string, id uint32, peer string) {
	msgChan, _ := gossiper.gossipHandler.MongeringChannels.LoadOrStore(peer, &sync.Map{})
	msgChan.(*sync.Map).Delete(MessageUniqueID{Origin: origin, ID: id})
}

// notify listeners if status received satify condition
func (gossiper *Gossiper) notifyListenersForStatus(extpacket *ExtendedGossipPacket) {
	msgChan, exists := gossiper.gossipHandler.MongeringChannels.Load(extpacket.SenderAddr.String())
	if exists {
		msgChan.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
			msg := key.(MessageUniqueID)
			channel := value.(chan bool)
			for _, ps := range extpacket.Packet.Status.Want {
				if ps.Identifier == msg.Origin && msg.ID < ps.NextID {
					go func(c chan bool) {
						c <- true
					}(channel)
				}
			}

			return true
		})
	}
}
