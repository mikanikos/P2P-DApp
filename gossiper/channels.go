package gossiper

import (
	"sync"
)

// MessageUniqueIdentifier struct
type MessageUniqueIdentifier struct {
	Origin string
	ID     uint32
}

func (gossiper *Gossiper) getListenerForStatus(origin string, id uint32, peer string) (chan bool, bool) {
	msgChan, _ := gossiper.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	channel, loaded := msgChan.(*sync.Map).LoadOrStore(MessageUniqueIdentifier{Origin: origin, ID: id}, make(chan bool, maxChannelSize))
	return channel.(chan bool), loaded
}

func (gossiper *Gossiper) deleteListenerForStatus(origin string, id uint32, peer string) {
	msgChan, _ := gossiper.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	msgChan.(*sync.Map).Delete(MessageUniqueIdentifier{Origin: origin, ID: id})
}

func (gossiper *Gossiper) notifyListenersForStatus(extpacket *ExtendedGossipPacket) {
	msgChan, exists := gossiper.mongeringChannels.Load(extpacket.SenderAddr.String())
	if exists {
		msgChan.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
			msg := key.(MessageUniqueIdentifier)
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
