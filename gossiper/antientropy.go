package gossiper

import (
	"time"
)

// StartAntiEntropy with the specified timer
func (gossiper *Gossiper) StartAntiEntropy(antiEntropyTimeout int) {
	if antiEntropyTimeout > 0 {
		timer := time.NewTicker(time.Duration(antiEntropyTimeout) * time.Second)
		for {
			select {
			case <-timer.C:
				peersCopy := gossiper.GetPeersAtomic()
				if len(peersCopy) != 0 {
					randomPeer := gossiper.getRandomPeer(peersCopy)
					statusToSend := gossiper.getStatusToSend()
					gossiper.sendPacket(statusToSend, randomPeer)
				}
			}
		}
	}
}
