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

func (gossiper *Gossiper) notifyMongeringChannel(peer string) {
	gossiper.mongeringChannels.Mutex.Lock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	_, channelCreated := gossiper.mongeringChannels.Channels[peer]
	if channelCreated {
		go func() {
			gossiper.mongeringChannels.Channels[peer] <- true
		}()
	}
}

func (gossiper *Gossiper) notifySyncChannel(peer string) {
	gossiper.syncChannels.Mutex.Lock()
	defer gossiper.syncChannels.Mutex.Unlock()

	_, channelCreated := gossiper.syncChannels.Channels[peer]
	if channelCreated {
		go func() {
			gossiper.syncChannels.Channels[peer] <- true
		}()
	}
}

func (gossiper *Gossiper) closeChannels(peer string) {
	gossiper.mongeringChannels.Mutex.Lock()
	gossiper.syncChannels.Mutex.Lock()

	defer gossiper.syncChannels.Mutex.Unlock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	close(gossiper.mongeringChannels.Channels[peer])
	close(gossiper.syncChannels.Channels[peer])
}

func (gossiper *Gossiper) checkMongeringForPeer(peer string) bool {
	gossiper.syncChannels.Mutex.Lock()
	gossiper.mongeringChannels.Mutex.Lock()

	defer gossiper.syncChannels.Mutex.Unlock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]

	_, syncChanPresent := gossiper.syncChannels.Channels[peer]
	if mongerChanPresent || syncChanPresent {
		return true
	}
	return false
}

func (gossiper *Gossiper) creatingRumorSyncChannels(peer string) {
	gossiper.mongeringChannels.Mutex.Lock()
	gossiper.syncChannels.Mutex.Lock()

	defer gossiper.mongeringChannels.Mutex.Unlock()
	defer gossiper.syncChannels.Mutex.Unlock()

	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]
	if !mongerChanPresent {
		gossiper.mongeringChannels.Channels[peer] = make(chan bool)
	}

	_, syncChanPresent := gossiper.syncChannels.Channels[peer]
	if !syncChanPresent {
		gossiper.syncChannels.Channels[peer] = make(chan bool)
	}
}

func (gossiper *Gossiper) sendToPeerStatusChannel(extPacket *ExtendedGossipPacket) {
	gossiper.statusChannels.Mutex.Lock()
	defer gossiper.statusChannels.Mutex.Unlock()

	_, exists := gossiper.statusChannels.Channels[extPacket.SenderAddr.String()]
	if !exists {
		gossiper.statusChannels.Channels[extPacket.SenderAddr.String()] = make(chan *ExtendedGossipPacket)
		go gossiper.handlePeerStatus(gossiper.statusChannels.Channels[extPacket.SenderAddr.String()])
	}
	gossiper.statusChannels.Channels[extPacket.SenderAddr.String()] <- extPacket
}
