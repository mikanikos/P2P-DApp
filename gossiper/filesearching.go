package gossiper

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// SafeRequestMap struct
type SafeRequestMap struct {
	OriginTimeMap map[string]time.Time
	Mutex         sync.RWMutex
}

// handle single search result
func (gossiper *Gossiper) handleSearchResult(origin string, res *SearchResult) {

	// save it if not present
	_, loaded := gossiper.fileHandler.hashDataMap.Load(string(res.MetafileHash))

	// check if I already have metafile information needed for chunks download
	if !loaded {
		printSearchMatchMessage(origin, res)

		// download metafile
		if !gossiper.downloadDataFromPeer(res.FileName, origin, res.MetafileHash, 0) {
			if debug {
				fmt.Println("ERROR: the peer doesn't have the metafile or is offline")
			}
			return
		}
	}

	metadataStored, loaded := gossiper.fileHandler.myFiles.Load(getKeyFromString(string(res.MetafileHash) + res.FileName))
	fileMetadata := metadataStored.(*FileMetadata)

	// store chunk owner origin and search result to gui
	for chunkID := range fileMetadata.ChunkMap.ChunkOwners {
		fileMetadata.updateChunkOwner(origin, chunkID)
	}
	gossiper.addSearchFileForGUI(fileMetadata)
}

// check if incoming search request is recent
func (gossiper *Gossiper) isRecentSearchRequest(searchRequest *SearchRequest) bool {

	// sort and get keywords single string as key
	sort.Strings(searchRequest.Keywords)
	identifier := getKeyFromString(searchRequest.Origin + strings.Join(searchRequest.Keywords, ""))
	stored := gossiper.fileHandler.lastSearchRequests.OriginTimeMap[identifier]

	// if not stored or timeout has expired, can take it otherwise no
	if stored.IsZero() || stored.Add(searchRequestDuplicateTimeout).Before(time.Now()) {

		// save new information and reset timer
		if gossiper.fileHandler.lastSearchRequests.OriginTimeMap[identifier].IsZero() {
			gossiper.fileHandler.lastSearchRequests.OriginTimeMap[identifier] = time.Now()
		} else {
			last := gossiper.fileHandler.lastSearchRequests.OriginTimeMap[identifier]
			last = time.Now()
			gossiper.fileHandler.lastSearchRequests.OriginTimeMap[identifier] = last
		}
		return false
	}
	return true
}

// search file with budget
func (gossiper *Gossiper) searchFilePeriodically(extPacket *ExtendedGossipPacket, increment bool) {

	if debug {
		fmt.Println("Got search request")
	}

	keywords := extPacket.Packet.SearchRequest.Keywords
	budget := extPacket.Packet.SearchRequest.Budget

	// decrement budget at source
	extPacket.Packet.SearchRequest.Budget = budget - 1

	if budget <= 0 {
		if debug {
			fmt.Println("Budget too low")
		}
		return
	}

	// forward request
	gossiper.forwardSearchRequestWithBudget(extPacket)

	// start timer
	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)
	for {
		select {
		case <-timer.C:

			// if budget too high. timeout
			if budget > uint64(maxBudget) {
				timer.Stop()
				if debug {
					fmt.Println("Search timeout")
				}
				return
			}

			// if reached matching threshold, search finished
			if gossiper.searchForFilesNotDownloaded(keywords) >= matchThreshold {
				timer.Stop()
				if hw3 {
					fmt.Println("SEARCH FINISHED")
				}
				return
			}

			// increment budget if it was not specified by argument
			if increment {
				extPacket.Packet.SearchRequest.Budget = budget * 2
				budget = extPacket.Packet.SearchRequest.Budget
			}
			if debug {
				fmt.Println("Sending request")
			}

			// send new request with updated budget
			extPacket.Packet.SearchRequest.Budget = budget - 1
			gossiper.forwardSearchRequestWithBudget(extPacket)
		}
	}
}

// search locally for files matches given keywords
func (gossiper *Gossiper) searchForFilesNotDownloaded(keywords []string) int {
	matches := make([]string, 0)

	// get over all the files
	gossiper.fileHandler.myFiles.Range(func(key interface{}, value interface{}) bool {
		fileMetadata := value.(*FileMetadata)
		// check if match at least one keyword, if all chunks locations are known and it's not a file I already have
		if containsKeyword(fileMetadata.FileName, keywords) && fileMetadata.checkAllChunksLocation() && !fileMetadata.Confirmed {
			matches = append(matches, fileMetadata.FileName)
			matches = helpers.RemoveDuplicatesFromSlice(matches)
		}

		// check if threshold reached and stop iterating in that case
		if len(matches) == matchThreshold {
			return false
		}
		return true
	})

	if debug {
		fmt.Println("Got " + fmt.Sprint(len(matches)))
	}
	return len(matches)
}

// process search request and send all matches
func (gossiper *Gossiper) sendMatchingLocalFiles(extPacket *ExtendedGossipPacket) {

	keywords := extPacket.Packet.SearchRequest.Keywords
	searchResults := make([]*SearchResult, 0)

	gossiper.fileHandler.myFiles.Range(func(key interface{}, value interface{}) bool {
		fileMetadata := value.(*FileMetadata)

		// check if match at least one keyword
		if containsKeyword(fileMetadata.FileName, keywords) {
			searchResults = append(searchResults, &SearchResult{FileName: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, ChunkCount: fileMetadata.ChunkCount, ChunkMap: fileMetadata.ChunkMap.getKeyList()})
		}

		return true
	})

	// if I have some results, send them back to request origin
	if len(searchResults) != 0 {
		if debug {
			fmt.Println("Sending search results")
		}
		searchReply := &SearchReply{Origin: gossiper.name, Destination: extPacket.Packet.SearchRequest.Origin, HopLimit: uint32(hopLimit), Results: searchResults}
		packetToSend := &GossipPacket{SearchReply: searchReply}

		go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.SearchReply.HopLimit, packetToSend.SearchReply.Destination)
	}

	// forward request with left budget
	extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1
	gossiper.forwardSearchRequestWithBudget(extPacket)

}

// forward request as evenly as possible to the known peers
func (gossiper *Gossiper) forwardSearchRequestWithBudget(extPacket *ExtendedGossipPacket) {

	peers := gossiper.GetPeersAtomic()
	peersChosen := []*net.UDPAddr{extPacket.SenderAddr}
	availablePeers := helpers.DifferenceString(peers, peersChosen)

	// if budget is valid and I know some other peers (don't sending back to the peers I received it)
	if extPacket.Packet.SearchRequest.Budget > 0 && len(availablePeers) != 0 {

		if debug {
			fmt.Println("Handling request from " + extPacket.SenderAddr.String() + " with total budget " + fmt.Sprint(extPacket.Packet.SearchRequest.Budget))
		}

		// compute minimal budget for each peer and budget to share among all the peers
		budgetForEach := extPacket.Packet.SearchRequest.Budget / uint64(len(availablePeers))
		budgetToShare := extPacket.Packet.SearchRequest.Budget % uint64(len(availablePeers))

		// repetedly get random peer, send minimal budget + 1 if some budget to share left
		for len(availablePeers) != 0 {

			if budgetToShare == 0 && budgetForEach == 0 {
				return
			}

			randomPeer := getRandomPeer(availablePeers)
			if budgetToShare > 0 {
				extPacket.Packet.SearchRequest.Budget = budgetForEach + 1
				budgetToShare = budgetToShare - 1
			} else {
				extPacket.Packet.SearchRequest.Budget = budgetForEach
			}

			if debug {
				fmt.Println("Sending request to " + randomPeer.String() + " with budget " + fmt.Sprint(extPacket.Packet.SearchRequest.Budget))
			}

			// send packet and remove current peer
			gossiper.sendPacket(extPacket.Packet, randomPeer)
			peersChosen = append(peersChosen, randomPeer)
			availablePeers = helpers.DifferenceString(gossiper.GetPeersAtomic(), peersChosen)
		}
	}
}

// add search result to gui
func (gossiper *Gossiper) addSearchFileForGUI(fileMetadata *FileMetadata) {

	element := FileGUI{Name: fileMetadata.FileName, MetaHash: string(fileMetadata.MetafileHash)}
	contains := false
	for _, elem := range gossiper.uiHandler.filesSearched {
		if elem.Name == element.Name && elem.MetaHash == element.MetaHash {
			contains = true
			break
		}
	}

	if !contains {
		gossiper.uiHandler.filesSearched = append(gossiper.uiHandler.filesSearched, element)
	}
}
