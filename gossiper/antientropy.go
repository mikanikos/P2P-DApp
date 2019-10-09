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
			//fmt.Println("Sending periodic status to " + randomPeer.String())
			gossiper.sendStatusPacket(randomPeer)
		}
	}
}
