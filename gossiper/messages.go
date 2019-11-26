package gossiper

import (
	"sync"
)

func (gossiper *Gossiper) storeMessage(packet *GossipPacket, origin string, id uint32) bool {

	value, _ := gossiper.messageStorage.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, packet)

	gossiper.updateStatus(origin, id, mapValue)

	return loaded
}

func (gossiper *Gossiper) updateStatus(origin string, id uint32, mapValue *sync.Map) {

	gossiper.myStatus.Mutex.Lock()
	defer gossiper.myStatus.Mutex.Unlock()

	value, peerExists := gossiper.myStatus.Status[origin]
	maxID := uint32(1)
	if peerExists {
		maxID = uint32(value)
	}

	if maxID <= id {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossiper.myStatus.Status[origin] = maxID
		}
	}
}

func (gossiper *Gossiper) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossiper.messageStorage.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return message.(*GossipPacket)
}
