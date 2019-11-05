package gossiper

import (
	"sync"
)

// PacketsStorage struct
type PacketsStorage struct {
	OriginPacketsMap sync.Map
	LatestMessages   chan *RumorMessage
}

func (gossiper *Gossiper) addMessage(extPacket *ExtendedGossipPacket) bool {

	var origin string
	var id uint32

	origin = extPacket.Packet.Rumor.Origin
	id = extPacket.Packet.Rumor.ID

	value, _ := gossiper.originPackets.OriginPacketsMap.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, extPacket.Packet.Rumor)
	if !loaded && extPacket.Packet.Rumor.Text != "" {
		go func(r *RumorMessage) {
			gossiper.originPackets.LatestMessages <- r
		}(extPacket.Packet.Rumor)
	}

	gossiper.myStatus.Mutex.Lock()
	defer gossiper.myStatus.Mutex.Unlock()

	value, peerExists := gossiper.myStatus.Status[extPacket.Packet.Rumor.Origin]
	maxID := uint32(1)
	if peerExists {
		maxID = value.(uint32)
	}

	if maxID <= extPacket.Packet.Rumor.ID {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossiper.myStatus.Status[origin] = maxID
		}
	}

	return loaded
}

// GetMessages for GUI
func (gossiper *Gossiper) GetMessages() []RumorMessage {

	bufferLength := len(gossiper.originPackets.LatestMessages)

	messages := make([]RumorMessage, bufferLength)
	for i := 0; i < bufferLength; i++ {
		message := <-gossiper.originPackets.LatestMessages
		messages[i] = *message
	}

	return messages
}

func (gossiper *Gossiper) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossiper.originPackets.OriginPacketsMap.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return &GossipPacket{Rumor: message.(*RumorMessage)}
}
