package gossiper

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) searchFilePeriodically(extPacket *ExtendedGossipPacket) {

	budget := extPacket.Packet.SearchRequest.Budget
	keywords := extPacket.Packet.SearchRequest.Keywords

	go gossiper.broadcastToPeers(extPacket)

	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)

	for budget < uint64(maxBudget) && gossiper.getMatchesForKeywords(keywords) < matchThreshold {
		select {
		case <-timer.C:
			go gossiper.broadcastToPeers(extPacket)
		}
	}
	timer.Stop()

	fmt.Println("SEARCH FINISHED")

}

func (gossiper *Gossiper) getMatchesForKeywords(keywords []string) int {

	matches := 0

	for _, k := range keywords {

		gossiper.myFiles.Range(func(key interface{}, value interface{}) bool {
			fileMetadata := value.(*FileMetadata)

			if strings.Contains(fileMetadata.FileSearchData.FileName, k) && fileMetadata.Size == 0 {
				metaFile := *fileMetadata.MetaFile
				knowAllTheChunks := true

				for i := uint64(0); i < fileMetadata.FileSearchData.ChunkCount; i++ {
					hash := metaFile[i*32 : (i+1)*32]
					value, loaded := gossiper.myFileChunks.Load(hash)
					if loaded {
						chunkOwnerEntry := value.(*ChunkOwners)
						if len(chunkOwnerEntry.Owners) == 0 {
							knowAllTheChunks = false
							break
						}
					} else {
						knowAllTheChunks = false
						break
					}
				}

				if knowAllTheChunks {
					matches++
				}
			}
			return true
		})
	}

	return matches
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

	go gossiper.forwardRequestWithBudget(extPacket)

}

func (gossiper *Gossiper) forwardRequestWithBudget(extPacket *ExtendedGossipPacket) {

	extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1

	if extPacket.Packet.SearchRequest.Budget > 0 {

		budget := extPacket.Packet.SearchRequest.Budget

		peers := gossiper.GetPeersAtomic()
		numberPeers := len(peers)

		budgetForEach := budget / uint64(numberPeers)
		budgetToShare := budget % uint64(numberPeers)

		peersChosen := make([]*net.UDPAddr, 0)
		availablePeers := peers

		for len(availablePeers) != 0 || (budgetToShare != 0 && budgetForEach == 0) {

			if budgetToShare > 0 {
				extPacket.Packet.SearchRequest.Budget = budgetForEach + 1
				budgetToShare = budgetToShare - 1
			} else {
				extPacket.Packet.SearchRequest.Budget = budgetForEach
			}

			randomPeer := gossiper.getRandomPeer(availablePeers)
			go gossiper.sendPacket(extPacket.Packet, randomPeer)

			peersChosen = append(peersChosen, randomPeer)
			availablePeers = helpers.DifferenceString(gossiper.GetPeersAtomic(), peersChosen)
		}
	}

}
