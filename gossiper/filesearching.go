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

func (gossiper *Gossiper) searchFilePeriodically(extPacket *ExtendedGossipPacket) {

	if debug {
		fmt.Println("Got search request")
	}

	keywords := extPacket.Packet.SearchRequest.Keywords

	gossiper.broadcastToPeers(extPacket)

	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)

	for {
		select {
		case <-timer.C:

			if gossiper.checkEnoughMatches(keywords) {
				timer.Stop()
				if hw3 {
					fmt.Println("SEARCH FINISHED")
				}
				return
			}

			if extPacket.Packet.SearchRequest.Budget >= uint64(maxBudget) {
				timer.Stop()

				if debug {
					fmt.Println("SEARCH TIMEOUT")
				}

				return
			}

			extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget * 2
			if debug {
				fmt.Println("Sending request")
			}
			gossiper.broadcastToPeers(extPacket)

		}
	}
	//timer.Stop()

}

func (gossiper *Gossiper) checkEnoughMatches(keywords []string) bool {

	matches := make([]string, 0)

	for _, k := range keywords {

		if debug {
			fmt.Println("Looking for " + k)
		}

		gossiper.myFiles.Range(func(key interface{}, value interface{}) bool {
			fileMetadata := value.(*FileMetadata)

			if strings.Contains(fileMetadata.FileSearchData.FileName, k) && fileMetadata.Size == 0 {
				metaFile := *fileMetadata.MetaFile
				knowAllTheChunks := true

				for i := uint64(0); i < fileMetadata.FileSearchData.ChunkCount; i++ {
					hash := metaFile[i*32 : (i+1)*32]
					value, loaded := gossiper.myFileChunks.Load(hex.EncodeToString(hash))
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
					matches = append(matches, fileMetadata.FileSearchData.FileName)
					matches = helpers.RemoveDuplicatesFromSlice(matches)
				}
			}

			if len(matches) == matchThreshold {
				return false
			}

			return true
		})

		if len(matches) == matchThreshold {
			return true
		}
	}

	return false
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
