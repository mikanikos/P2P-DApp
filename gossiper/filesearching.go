package gossiper

import (
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// MutexSearchResult struct
type MutexSearchResult struct {
	SearchResults map[string]time.Time
	Mutex         sync.RWMutex
}

func (gossiper *Gossiper) isRecentSearchRequest(searchRequest *SearchRequest) bool {

	sort.Strings(searchRequest.Keywords)
	identifier := searchRequest.Origin + strings.Join(searchRequest.Keywords, "")
	gossiper.lastSearchRequests.Mutex.RLock()
	stored := gossiper.lastSearchRequests.SearchResults[identifier]
	gossiper.lastSearchRequests.Mutex.RUnlock()

	if stored.IsZero() || stored.Add(searchRequestDuplicate).Before(time.Now()) {
		gossiper.lastSearchRequests.Mutex.Lock()
		defer gossiper.lastSearchRequests.Mutex.Unlock()

		if gossiper.lastSearchRequests.SearchResults[identifier].IsZero() {
			gossiper.lastSearchRequests.SearchResults[identifier] = time.Now()
		} else {
			last := gossiper.lastSearchRequests.SearchResults[identifier]
			last = time.Now()
			gossiper.lastSearchRequests.SearchResults[identifier] = last
		}
		return false
	}
	return true
}

func (gossiper *Gossiper) searchFilePeriodically(extPacket *ExtendedGossipPacket, increment bool) {

	if debug {
		fmt.Println("Got search request")
	}

	keywords := extPacket.Packet.SearchRequest.Keywords
	budget := extPacket.Packet.SearchRequest.Budget

	extPacket.Packet.SearchRequest.Budget = budget - 1

	if budget <= 0 {
		if debug {
			fmt.Println("Budget too low")
		}
		return
	}

	gossiper.broadcastToPeers(extPacket)

	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)

	for {
		select {
		case <-timer.C:

			if extPacket.Packet.SearchRequest.Budget >= uint64(maxBudget) {
				timer.Stop()

				if debug {
					fmt.Println("Search timeout")
				}

				return
			}

			if gossiper.searchForFilesNotDownloaded(keywords) >= matchThreshold {
				timer.Stop()
				if hw3 {
					fmt.Println("SEARCH FINISHED")
				}
				return
			}

			if increment {
				extPacket.Packet.SearchRequest.Budget = budget * 2
				budget = extPacket.Packet.SearchRequest.Budget
			}
			if debug {
				fmt.Println("Sending request")
			}
			extPacket.Packet.SearchRequest.Budget = budget - 1
			gossiper.broadcastToPeers(extPacket)

		}
	}
	//timer.Stop()

}

func (gossiper *Gossiper) searchForFilesNotDownloaded(keywords []string) int {

	matches := make([]string, 0)

	for _, k := range keywords {

		if debug {
			fmt.Println("Looking for " + k)
		}

		gossiper.filesList.Range(func(key interface{}, value interface{}) bool {
			id := value.(*FileIDPair)

			if strings.Contains(id.FileName, k) {

				valueFile, loaded := gossiper.myFiles.Load(id.EncMetaHash)

				if !loaded {
					if debug {
						fmt.Println("Error: file not found when searching for search results")
					}
					return false
				}

				fileMetadata := valueFile.(*FileMetadata)

				metaFile := *fileMetadata.MetaFile

				if fileMetadata.Size == 0 {

					knowAllTheChunks := true

					for i := uint64(0); i < fileMetadata.FileSearchData.ChunkCount; i++ {
						hash := metaFile[i*32 : (i+1)*32]
						chunkValue, loaded := gossiper.myFileChunks.Load(hex.EncodeToString(hash))
						if loaded {
							chunkOwnerEntry := chunkValue.(*ChunkOwners)
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
						hashName := key.(string)
						matches = append(matches, hashName)
						matches = helpers.RemoveDuplicatesFromSlice(matches)
					}
				}
			}

			return true
		})
	}

	return len(matches)
}

func (gossiper *Gossiper) sendMatchingLocalFiles(extPacket *ExtendedGossipPacket) {

	keywords := extPacket.Packet.SearchRequest.Keywords
	searchResults := make([]*SearchResult, 0)

	for _, k := range keywords {

		gossiper.filesList.Range(func(key interface{}, value interface{}) bool {
			id := value.(*FileIDPair)

			if strings.Contains(id.FileName, k) {
				valueFile, loaded := gossiper.myFiles.Load(id.EncMetaHash)

				if !loaded {
					if debug {
						fmt.Println("Error: file not found when searching for search results")
					}
					return false
				}

				fileMetadata := valueFile.(*FileMetadata)
				searchResults = append(searchResults, &SearchResult{FileName: id.FileName, MetafileHash: fileMetadata.FileSearchData.MetafileHash, ChunkCount: fileMetadata.FileSearchData.ChunkCount, ChunkMap: fileMetadata.FileSearchData.ChunkMap})
			}

			return true
		})

		// gossiper.myFiles.Range(func(key interface{}, value interface{}) bool {
		// 	fileMetadata := value.(*FileMetadata)
		// 	if strings.Contains(fileMetadata.FileSearchData.FileName, k) {
		// 		searchResults = append(searchResults, fileMetadata.FileSearchData)
		// 	}
		// 	return true
		// })
	}

	if len(searchResults) != 0 {

		if debug {
			fmt.Println("Sending search results")
		}

		searchReply := &SearchReply{Origin: gossiper.name, Destination: extPacket.Packet.SearchRequest.Origin, HopLimit: uint32(hopLimit), Results: searchResults}
		packetToSend := &GossipPacket{SearchReply: searchReply}

		go gossiper.forwardPrivateMessage(packetToSend)
	}

	go gossiper.forwardRequestWithBudget(extPacket)

}

func (gossiper *Gossiper) forwardRequestWithBudget(extPacket *ExtendedGossipPacket) {

	extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1

	if extPacket.Packet.SearchRequest.Budget > 0 {

		if debug {
			fmt.Println("Forwarding request from " + extPacket.SenderAddr.String())
		}

		budget := extPacket.Packet.SearchRequest.Budget

		peers := gossiper.GetPeersAtomic()
		numberPeers := len(peers)

		budgetForEach := budget / uint64(numberPeers)
		budgetToShare := budget % uint64(numberPeers)

		peersChosen := make([]*net.UDPAddr, 0)
		peersChosen = append(peersChosen, extPacket.SenderAddr)
		availablePeers := helpers.DifferenceString(peers, peersChosen)

		for len(availablePeers) != 0 || (budgetToShare != 0 && budgetForEach == 0) {

			randomPeer := gossiper.getRandomPeer(availablePeers)

			if budgetToShare > 0 {
				extPacket.Packet.SearchRequest.Budget = budgetForEach + 1
				budgetToShare = budgetToShare - 1
			} else {
				extPacket.Packet.SearchRequest.Budget = budgetForEach
			}

			gossiper.sendPacket(extPacket.Packet, randomPeer)

			peersChosen = append(peersChosen, randomPeer)
			availablePeers = helpers.DifferenceString(gossiper.GetPeersAtomic(), peersChosen)
		}
	}

}

// GetFilesSearched for GUI
func (gossiper *Gossiper) GetFilesSearched() []FileGUI {
	return gossiper.filesSearched
}
