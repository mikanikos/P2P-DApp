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

	_, loaded := mapValue.LoadOrStore(id, extPacket.Packet)
	if !loaded && extPacket.Packet.Rumor.Text != "" {
		gossiper.originPackets.LatestMessages <- extPacket.Packet.Rumor
	}

	return loaded
}

// GetMessages either in simple or normal mode
func (gossiper *Gossiper) GetMessages() []RumorMessage {

	bufferLength := len(gossiper.originPackets.LatestMessages)

	messages := make([]RumorMessage, bufferLength)
	for i := 0; i < bufferLength; i++ {
		message := <-gossiper.originPackets.LatestMessages
		messages[i] = *message
	}

	return messages
}
