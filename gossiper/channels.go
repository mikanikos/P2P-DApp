package gossiper

import (
	"sync"
)

// MutexDummyChannel struct
type MutexDummyChannel struct {
	Channels map[string]chan bool
	Mutex    sync.Mutex
}

func initializeChannels(modeTypes []string) (channels map[string]chan *ExtendedGossipPacket) {
	channels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		channels[t] = make(chan *ExtendedGossipPacket)
	}
	return channels
}

func (gossiper *Gossiper) notifyMongerChannel(peer string) {
	gossiper.mongeringChannels.Mutex.Lock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	rumorChan, channelCreated := gossiper.mongeringChannels.Channels[peer]
	if channelCreated {
		close(rumorChan)
	}
	gossiper.mongeringChannels.Channels[peer] = make(chan bool, 0)
}

func (gossiper *Gossiper) createOrGetMongerChannel(peer string) chan bool {
	gossiper.mongeringChannels.Mutex.Lock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]
	if !mongerChanPresent {
		gossiper.mongeringChannels.Channels[peer] = make(chan bool, 0)
	}

	return gossiper.mongeringChannels.Channels[peer]
}

func (gossiper *Gossiper) sendToPeerStatusChannel(extPacket *ExtendedGossipPacket) {

	channelPeer, exists := gossiper.statusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket))
	if !exists {
		go gossiper.handlePeerStatus(channelPeer.(chan *ExtendedGossipPacket))
	}
	channelPeer.(chan *ExtendedGossipPacket) <- extPacket
}
