package gossiper

import (
	"math/rand"
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
					indexPeer := rand.Intn(len(peersCopy))
					randomPeer := peersCopy[indexPeer]
					gossiper.sendStatusPacket(randomPeer)
				}
			}
		}
	}
}
