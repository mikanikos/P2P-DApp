package gossiper

import (
	"sync"
)

// MutexDummyChannel struct
// type MutexDummyChannel struct {
// 	Channels map[string]chan bool
// 	Mutex    sync.Mutex
// }

func initializeChannels(modeTypes []string, simpleMode bool) (channels map[string]chan *ExtendedGossipPacket) {
	channels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		if (t != "simple" && !simpleMode) || (t == "simple" && simpleMode) {
			channels[t] = make(chan *ExtendedGossipPacket)
		}
	}
	return channels
}

// func (gossiper *Gossiper) notifyMongerChannel(peer string) {
// 	gossiper.mongeringChannels.Mutex.Lock()
// 	defer gossiper.mongeringChannels.Mutex.Unlock()

// 	rumorChan, channelCreated := gossiper.mongeringChannels.Channels[peer]
// 	if channelCreated {
// 		close(rumorChan)
// 	}
// 	gossiper.mongeringChannels.Channels[peer] = make(chan bool, 0)
// }

// func (gossiper *Gossiper) createOrGetMongerChannel(peer string) chan bool {
// 	gossiper.mongeringChannels.Mutex.Lock()
// 	defer gossiper.mongeringChannels.Mutex.Unlock()

// 	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]
// 	if !mongerChanPresent {
// 		gossiper.mongeringChannels.Channels[peer] = make(chan bool, 0)
// 	}

// 	return gossiper.mongeringChannels.Channels[peer]
// }

// MessageUniqueIdentifier struct
type MessageUniqueIdentifier struct {
	Origin string
	ID     uint32
}

func (gossiper *Gossiper) getListenerForStatus(packet *GossipPacket, peer string) (chan bool, bool) {
	msgChan, _ := gossiper.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	channel, loaded := msgChan.(*sync.Map).LoadOrStore(&MessageUniqueIdentifier{Origin: packet.Rumor.Origin, ID: packet.Rumor.ID}, make(chan bool))
	return channel.(chan bool), loaded
}

func (gossiper *Gossiper) deleteListenerForStatus(packet *GossipPacket, peer string) {
	msgChan, _ := gossiper.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	msgChan.(*sync.Map).Delete(&MessageUniqueIdentifier{Origin: packet.Rumor.Origin, ID: packet.Rumor.ID})
}

func (gossiper *Gossiper) notifyListenersForStatus(extpacket *ExtendedGossipPacket) {
	msgChan, exists := gossiper.mongeringChannels.Load(extpacket.SenderAddr.String())
	if exists {
		msgChan.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
			msg := key.(*MessageUniqueIdentifier)
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
