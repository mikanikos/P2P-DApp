package gossiper

import (
	"hash/fnv"
	"sync"
	"time"
)

// PacketsStorage struct
type PacketsStorage struct {
	OriginPacketsMap map[string]map[uint32]*ExtendedGossipPacket
	Messages         []RumorMessage
	Mutex            sync.Mutex
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func (gossiper *Gossiper) addMessage(extPacket *ExtendedGossipPacket) bool {

	var origin string
	var id uint32
	var msg string

	if gossiper.simpleMode {
		origin = extPacket.Packet.Simple.OriginalName
		id = hash(origin + time.Now().String())
		msg = extPacket.Packet.Simple.Contents
	} else {
		origin = extPacket.Packet.Rumor.Origin
		id = extPacket.Packet.Rumor.ID
		msg = extPacket.Packet.Rumor.Text
	}

	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()

	_, isPeerKnown := gossiper.originPackets.OriginPacketsMap[origin]
	if !isPeerKnown {
		gossiper.originPackets.OriginPacketsMap[origin] = make(map[uint32]*ExtendedGossipPacket)
	}
	_, isMessageKnown := gossiper.originPackets.OriginPacketsMap[origin][id]
	if !isMessageKnown {
		gossiper.originPackets.OriginPacketsMap[origin][id] = extPacket
		gossiper.originPackets.Messages = append(gossiper.originPackets.Messages, RumorMessage{ID: id, Origin: origin, Text: msg})
	}

	return isMessageKnown
}

// GetMessages either in simple or normal mode
func (gossiper *Gossiper) GetMessages() []RumorMessage {

	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	messagesCopy := make([]RumorMessage, len(gossiper.originPackets.Messages))
	copy(messagesCopy, gossiper.originPackets.Messages)
	gossiper.originPackets.Messages = make([]RumorMessage, 0)
	return messagesCopy
}
