package gossiper

import (
	"time"
)

func (gossiper *Gossiper) startAntiEntropy() {
	if gossiper.antiEntropyTimeout > 0 {
		timer := time.NewTicker(time.Duration(gossiper.antiEntropyTimeout) * time.Second)
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
