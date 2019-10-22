package gossiper

import (
	"sync"
)

// PacketsStorage struct
type PacketsStorage struct {
	OriginPacketsMap sync.Map //map[string]map[uint32]*ExtendedGossipPacket
	LatestMessages   chan *RumorMessage
	//Mutex            sync.Mutex
}

// func hash(s string) uint32 {
// 	h := fnv.New32a()
// 	h.Write([]byte(s))
// 	return h.Sum32()
// }

func (gossiper *Gossiper) addMessage(extPacket *ExtendedGossipPacket) bool {

	var origin string
	var id uint32

	origin = extPacket.Packet.Rumor.Origin
	id = extPacket.Packet.Rumor.ID

	// gossiper.originPackets.Mutex.Lock()
	// defer gossiper.originPackets.Mutex.Unlock()

	value, _ := gossiper.originPackets.OriginPacketsMap.LoadOrStore(origin, &sync.Map{})
	mapValue := value.(*sync.Map)

	_, loaded := mapValue.LoadOrStore(id, extPacket.Packet)
	if !loaded {
		gossiper.originPackets.LatestMessages <- extPacket.Packet.Rumor
	}

	// _, isPeerKnown := gossiper.originPackets.OriginPacketsMap[origin]
	// if !isPeerKnown {
	// 	gossiper.originPackets.OriginPacketsMap[origin] = make(map[uint32]*ExtendedGossipPacket)
	// }
	// _, isMessageKnown := gossiper.originPackets.OriginPacketsMap[origin][id]
	// if !isMessageKnown {
	// 	gossiper.originPackets.OriginPacketsMap[origin][id] = extPacket
	// 	gossiper.originPackets.Messages = append(gossiper.originPackets.Messages, RumorMessage{ID: id, Origin: origin, Text: msg})
	// }

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

	// gossiper.originPackets.Mutex.Lock()
	// defer gossiper.originPackets.Mutex.Unlock()
	// messagesCopy := make([]RumorMessage, len(gossiper.originPackets.Messages))
	// copy(messagesCopy, gossiper.originPackets.Messages)
	// gossiper.originPackets.Messages = make([]RumorMessage, 0)
	// return messagesCopy
}
