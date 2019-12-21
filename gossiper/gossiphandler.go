package gossiper

import (
	"sync"
)

// GossipHandler struct
type GossipHandler struct {
	SeqID             uint32
	MessageStorage    sync.Map
	MyStatus          *VectorClock
	StatusChannels    sync.Map
	MongeringChannels sync.Map
}

// NewGossipHandler create new routing handler
func NewGossipHandler() *GossipHandler {
	return &GossipHandler{
		SeqID:             1,
		MessageStorage:    sync.Map{},
		MyStatus:          &VectorClock{Entries: make(map[string]uint32)},
		StatusChannels:    sync.Map{},
		MongeringChannels: sync.Map{},
	}
}

// VectorClock struct
type VectorClock struct {
	Entries map[string]uint32
	Mutex   sync.RWMutex
}

func (gossiper *Gossiper) storeMessage(packet *GossipPacket, origin string, id uint32) bool {

	value, _ := gossiper.gossipHandler.MessageStorage.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, packet)

	gossiper.updateStatus(origin, id, mapValue)

	return loaded
}

func (gossiper *Gossiper) updateStatus(origin string, id uint32, mapValue *sync.Map) {

	gossiper.gossipHandler.MyStatus.Mutex.Lock()
	defer gossiper.gossipHandler.MyStatus.Mutex.Unlock()

	value, peerExists := gossiper.gossipHandler.MyStatus.Entries[origin]
	maxID := uint32(1)
	if peerExists {
		maxID = uint32(value)
	}

	if maxID <= id {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossiper.gossipHandler.MyStatus.Entries[origin] = maxID
		}
	}
}

func (gossiper *Gossiper) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossiper.gossipHandler.MessageStorage.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return message.(*GossipPacket)
}

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

func (gossiper *Gossiper) getListenerForStatus(origin string, id uint32, peer string) (chan bool, bool) {
	msgChan, _ := gossiper.gossipHandler.MongeringChannels.LoadOrStore(peer, &sync.Map{})
	channel, loaded := msgChan.(*sync.Map).LoadOrStore(MessageUniqueID{Origin: origin, ID: id}, make(chan bool, maxChannelSize))
	return channel.(chan bool), loaded
}

func (gossiper *Gossiper) deleteListenerForStatus(origin string, id uint32, peer string) {
	msgChan, _ := gossiper.gossipHandler.MongeringChannels.LoadOrStore(peer, &sync.Map{})
	msgChan.(*sync.Map).Delete(MessageUniqueID{Origin: origin, ID: id})
}

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
