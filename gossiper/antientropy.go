package gossiper

import (
	"math/rand"
	"time"
)

func (gossiper *Gossiper) startAntiEntropy() {
	timer := time.NewTicker(time.Duration(gossiper.antiEntropyTimeout) * time.Second)
	for {
		select {
		case <-timer.C:
			peersCopy := gossiper.GetPeersAtomic()
			indexPeer := rand.Intn(len(peersCopy))
			randomPeer := peersCopy[indexPeer]
			gossiper.sendStatusPacket(randomPeer)
		}
	}
}
