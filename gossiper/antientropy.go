package gossiper

import (
	"time"
)

// StartAntiEntropy with the specified timer
func (gossiper *Gossiper) StartAntiEntropy(antiEntropyTimeout uint) {
	if antiEntropyTimeout > 0 {
		timer := time.NewTicker(time.Duration(antiEntropyTimeout) * time.Second)
		for {
			select {
			case <-timer.C:
				peersCopy := gossiper.GetPeersAtomic()
				if len(peersCopy) != 0 {
					randomPeer := getRandomPeer(peersCopy)
					statusToSend := getStatusToSend(&gossiper.myStatus)
					gossiper.sendPacket(&GossipPacket{Status: statusToSend}, randomPeer)
				}
			}
		}
	}
}
