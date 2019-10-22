package gossiper

import (
	"sync"
)

// MutexStatusChannel struct
type MutexStatusChannel struct {
	Channels map[string]chan *ExtendedGossipPacket
	Mutex    sync.Mutex
}

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
	rumorChan, found := gossiper.mongeringChannels.Load(peer)
	if found {
		close(rumorChan.(chan bool))
	}
	gossiper.mongeringChannels.Store(peer, make(chan bool, 0))
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

// func (gossiper *Gossiper) notifySyncChannel(peer string) {
// 	gossiper.syncChannels.Mutex.Lock()
// 	defer gossiper.syncChannels.Mutex.Unlock()

// 	_, channelCreated := gossiper.syncChannels.Channels[peer]
// 	if channelCreated {
// 		go func() {
// 			gossiper.syncChannels.Channels[peer] <- true
// 		}()
// 	}
// }

// func (gossiper *Gossiper) closeRumorChannel(peer string) {
// 	gossiper.mongeringChannels.Mutex.Lock()
// 	defer gossiper.mongeringChannels.Mutex.Unlock()
// 	close(gossiper.mongeringChannels.Channels[peer])
// }

// func (gossiper *Gossiper) closeSyncChannel(peer string) {
// 	gossiper.syncChannels.Mutex.Lock()
// 	defer gossiper.syncChannels.Mutex.Unlock()
// 	close(gossiper.syncChannels.Channels[peer])
// }

// func (gossiper *Gossiper) checkMongeringForPeer(peer string) bool {
// 	gossiper.syncChannels.Mutex.Lock()
// 	gossiper.mongeringChannels.Mutex.Lock()

// 	defer gossiper.syncChannels.Mutex.Unlock()
// 	defer gossiper.mongeringChannels.Mutex.Unlock()

// 	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]

// 	_, syncChanPresent := gossiper.syncChannels.Channels[peer]
// 	if mongerChanPresent || syncChanPresent {
// 		return true
// 	}
// 	return false
// }

// func (gossiper *Gossiper) creatingRumorSyncChannels(peer string) {
// 	gossiper.mongeringChannels.Mutex.Lock()
// 	gossiper.syncChannels.Mutex.Lock()

// 	defer gossiper.mongeringChannels.Mutex.Unlock()
// 	defer gossiper.syncChannels.Mutex.Unlock()

// 	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]
// 	if !mongerChanPresent {
// 		gossiper.mongeringChannels.Channels[peer] = make(chan bool)
// 	}

// 	_, syncChanPresent := gossiper.syncChannels.Channels[peer]
// 	if !syncChanPresent {
// 		gossiper.syncChannels.Channels[peer] = make(chan bool)
// 	}
// }

func (gossiper *Gossiper) sendToPeerStatusChannel(extPacket *ExtendedGossipPacket) {
	// gossiper.statusChannels.Mutex.Lock()
	// defer gossiper.statusChannels.Mutex.Unlock()

	channelPeer, exists := gossiper.statusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket))
	if !exists {
		go gossiper.handlePeerStatus(channelPeer.(chan *ExtendedGossipPacket))
	}
	channelPeer.(chan *ExtendedGossipPacket) <- extPacket
}
