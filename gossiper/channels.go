package gossiper

import (
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
		//fmt.Println("Sending on channel monger for " + extPacket.SenderAddr.String())
		go func() {
			gossiper.mongeringChannels.Channels[extPacket.SenderAddr.String()] <- extPacket
		}()
	}
}

func (gossiper *Gossiper) notifySyncChannel(extPacket *ExtendedGossipPacket) {
	gossiper.syncChannels.Mutex.Lock()
	defer gossiper.syncChannels.Mutex.Unlock()

	_, channelCreated := gossiper.syncChannels.Channels[extPacket.SenderAddr.String()]
	if channelCreated {
		//fmt.Println("Sending on channel sync for " + extPacket.SenderAddr.String())
		go func() {
			gossiper.syncChannels.Channels[extPacket.SenderAddr.String()] <- extPacket
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
	//fmt.Println("Channels closed")
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

	//fmt.Println("Creating channels")

	_, mongerChanPresent := gossiper.mongeringChannels.Channels[peer]
	if !mongerChanPresent {
		// mongering channel
		gossiper.mongeringChannels.Channels[peer] = make(chan *ExtendedGossipPacket)
	}

	_, syncChanPresent := gossiper.syncChannels.Channels[peer]
	if !syncChanPresent {
		// sync channel
		gossiper.syncChannels.Channels[peer] = make(chan *ExtendedGossipPacket)
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
