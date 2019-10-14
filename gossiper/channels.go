package gossiper

import (
	"fmt"
	"sync"
)

// MutexStatusChannels struct
type MutexStatusChannels struct {
	Channels map[string]chan *ExtendedGossipPacket
	Mutex    sync.RWMutex
}

func initializeChannels(modeTypes []string) (channels map[string]chan *ExtendedGossipPacket) {
	channels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		channels[t] = make(chan *ExtendedGossipPacket)
	}
	return channels
}

func (gossiper *Gossiper) notifyMongeringChannel(extPacket *ExtendedGossipPacket) {
	gossiper.mongeringChannels.Mutex.Lock()
	defer gossiper.mongeringChannels.Mutex.Unlock()

	_, channelCreated := gossiper.mongeringChannels.Channels[extPacket.SenderAddr.String()]
	if channelCreated {
		go func() {
			gossiper.mongeringChannels.Channels[extPacket.SenderAddr.String()] <- extPacket
		}()
	}
}

func (gossiper *Gossiper) notifySyncChannel(extPacket *ExtendedGossipPacket) {
	gossiper.syncChannels.Mutex.Lock()
	defer gossiper.syncChannels.Mutex.Unlock()

	fmt.Println("Notifying " + extPacket.SenderAddr.String())

	_, channelCreated := gossiper.syncChannels.Channels[extPacket.SenderAddr.String()]
	if channelCreated {
		go func() {
			gossiper.syncChannels.Channels[extPacket.SenderAddr.String()] <- extPacket
		}()
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
