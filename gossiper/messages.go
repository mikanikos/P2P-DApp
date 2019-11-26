package gossiper

import (
	"sync"
)

func (gossiper *Gossiper) storeRumorMessage(rumor *RumorMessage) bool {

	value, _ := gossiper.rumorMessages.LoadOrStore(rumor.Origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(rumor.ID, rumor)
	if !loaded && rumor.Text != "" {
		go func(r *RumorMessage) {
			gossiper.uiHandler.latestRumors <- r
		}(rumor)
	}

	gossiper.updateStatus(rumor, mapValue)

	return loaded
}

func (gossiper *Gossiper) updateStatus(rumor *RumorMessage, mapValue *sync.Map) {

	gossiper.myRumorStatus.Mutex.Lock()
	defer gossiper.myRumorStatus.Mutex.Unlock()

	value, peerExists := gossiper.myRumorStatus.Status[rumor.Origin]
	maxID := uint32(1)
	if peerExists {
		maxID = uint32(value)
	}

	if maxID <= rumor.ID {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossiper.myRumorStatus.Status[rumor.Origin] = maxID
		}
	}
}

func (gossiper *Gossiper) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossiper.rumorMessages.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return &GossipPacket{Rumor: message.(*RumorMessage)}
}
