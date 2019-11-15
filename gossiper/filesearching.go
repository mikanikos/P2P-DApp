package gossiper

import (
	"fmt"
	"time"
)

func (gossiper *Gossiper) searchFilePeriodically(extPacket *ExtendedGossipPacket) {

	gossiper.broadcastToPeers(extPacket)
	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)
	for extPacket.Packet.SearchRequest.Budget < uint64(maxBudget) && gossiper.getMatchesForKeywords(extPacket.Packet.SearchRequest.Keywords) < matchThreshold {
		select {
		case <-timer.C:
			go gossiper.broadcastToPeers(extPacket)
		}
	}

	fmt.Println("SEARCH FINISHED")

}

func (gossiper *Gossiper) getMatchesForKeywords(keywords []string) int {

	// for _, k := range keywords {

	// }

	return 0

}

func (gossiper *Gossiper) searchFileLocally(extPacket *ExtendedGossipPacket) {

	// keywords := extPacket.Packet.SearchRequest.Keywords

	// for _, k := range keywords {

	// 	gossiper.retrieveAllFilesFromKeyword(k)

	// }

}
