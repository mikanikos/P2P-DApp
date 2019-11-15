package gossiper

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mikanikos/Peerster/helpers"
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

	// 	gossiper.myFiles.Range(func(key interface{}, value interface{}) bool {
	// 		fileMetadata := value.(*FileMetadata)

	// 	}

	// }

	return 0

}

func (gossiper *Gossiper) sendMatchingLocalFiles(extPacket *ExtendedGossipPacket) {

	keywords := extPacket.Packet.SearchRequest.Keywords
	searchResults := make([]*SearchResult, 0)

	for _, k := range keywords {

		gossiper.myFiles.Range(func(key interface{}, value interface{}) bool {
			fileMetadata := value.(*FileMetadata)
			if strings.Contains(fileMetadata.FileSearchData.FileName, k) {
				searchResults = append(searchResults, fileMetadata.FileSearchData)
			}
			return true
		})
	}

	if len(searchResults) != 0 {
		searchReply := &SearchReply{Origin: gossiper.name, Destination: extPacket.Packet.SearchRequest.Origin, HopLimit: uint32(hopLimit), Results: searchResults}
		packetToSend := &GossipPacket{SearchReply: searchReply}

		go gossiper.forwardPrivateMessage(packetToSend)
	}

	extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1

	if extPacket.Packet.SearchRequest.Budget > 0 {

		budget := extPacket.Packet.SearchRequest.Budget

		gossiper.peers.Mutex.RLock()
		peersLeft := gossiper.peers.Peers
		gossiper.peers.Mutex.RUnlock()

		numberPeers := len(peersLeft)
		budgetForEach := budget / uint64(numberPeers)
		budgetToShare := budget % uint64(numberPeers)

		for budgetToShare != 0 && budgetForEach != 0 {

			peer := gossiper.getRandomPeer(peersLeft)
			if budgetToShare > 0 {
				extPacket.Packet.SearchRequest.Budget = budgetForEach + 1
				budgetToShare = budgetToShare - 1
			} else {
				extPacket.Packet.SearchRequest.Budget = budgetForEach
			}

			peersWithRumor = []*net.UDPAddr{randomPeer}
			availablePeers := helpers.DifferenceString(peers, peersWithRumor)
			go gossiper.sendPacket()
		}
	}

}
