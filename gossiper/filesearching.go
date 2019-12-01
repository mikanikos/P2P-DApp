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

func (gossiper *Gossiper) handleSearchResult(origin string, res *SearchResult) {

	value, _ := gossiper.fileHandler.myFiles.LoadOrStore(hex.EncodeToString(res.MetafileHash), &FileMetadata{FileName: res.FileName, MetafileHash: res.MetafileHash, ChunkCount: res.ChunkCount, ChunkMap: make([]uint64, 0)})
	_, loaded := gossiper.fileHandler.filesList.LoadOrStore(hex.EncodeToString(res.MetafileHash)+res.FileName, &FileIDPair{FileName: res.FileName, EncMetaHash: hex.EncodeToString(res.MetafileHash)})
	fileMetadata := value.(*FileMetadata)

	if !loaded {
		printSearchMatchMessage(origin, res)
		gossiper.downloadMetafile(origin, fileMetadata)
	}

	gossiper.storeChunksOwner(origin, res.ChunkMap, fileMetadata)

	gossiper.addSearchFileForGUI(fileMetadata)
}

func (gossiper *Gossiper) isRecentSearchRequest(searchRequest *SearchRequest) bool {

	sort.Strings(searchRequest.Keywords)
	identifier := searchRequest.Origin + strings.Join(searchRequest.Keywords, "")
	stored := gossiper.fileHandler.lastSearchRequests.SearchResults[identifier]

	if stored.IsZero() || stored.Add(searchRequestDuplicateTimeout).Before(time.Now()) {

		if gossiper.fileHandler.lastSearchRequests.SearchResults[identifier].IsZero() {
			gossiper.fileHandler.lastSearchRequests.SearchResults[identifier] = time.Now()
		} else {
			last := gossiper.fileHandler.lastSearchRequests.SearchResults[identifier]
			last = time.Now()
			gossiper.fileHandler.lastSearchRequests.SearchResults[identifier] = last
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

	gossiper.forwardSearchRequestWithBudget(extPacket)

	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)
	for {
		select {
		case <-timer.C:

			if budget > uint64(maxBudget) {
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
			gossiper.forwardSearchRequestWithBudget(extPacket)
		}
	}
}

func (gossiper *Gossiper) searchForFilesNotDownloaded(keywords []string) int {

	matches := make([]string, 0)

	gossiper.fileHandler.filesList.Range(func(key interface{}, value interface{}) bool {
		//fileMetadata := value.(*FileMetadata)
		id := value.(*FileIDPair)
		if containsKeyword(id.FileName, keywords) {
			valueFile, loaded := gossiper.fileHandler.myFiles.Load(id.EncMetaHash)
			if !loaded {
				if debug {
					panic("Error: file not found when searching for search results")
				}
				return false
			}
			fileMetadata := valueFile.(*FileMetadata)

			//metaFile := *fileMetadata.MetaFile

			if len(fileMetadata.ChunkMap) == 0 { //&& gossiper.checkAllChunksLocation(metaFile, fileMetadata.ChunkCount) {
				//hashName := key.(string)
				matches = append(matches, fileMetadata.FileName)
				matches = helpers.RemoveDuplicatesFromSlice(matches)
			}
		}

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

func (gossiper *Gossiper) sendMatchingLocalFiles(extPacket *ExtendedGossipPacket) {

	keywords := extPacket.Packet.SearchRequest.Keywords
	searchResults := make([]*SearchResult, 0)

	gossiper.fileHandler.filesList.Range(func(key interface{}, value interface{}) bool {
		//fileMetadata := value.(*FileMetadata)
		id := value.(*FileIDPair)
		if containsKeyword(id.FileName, keywords) {
			valueFile, loaded := gossiper.fileHandler.myFiles.Load(id.EncMetaHash)
			if !loaded {
				if debug {
					panic("Error: file not found when searching for search results")
				}
				return false
			}
			fileMetadata := valueFile.(*FileMetadata)
			if len(fileMetadata.ChunkMap) != 0 {
				searchResults = append(searchResults, &SearchResult{FileName: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, ChunkCount: fileMetadata.ChunkCount, ChunkMap: fileMetadata.ChunkMap})
			}
		}
		return true
	})

	if len(searchResults) != 0 {
		if debug {
			fmt.Println("Sending search results")
		}
		searchReply := &SearchReply{Origin: gossiper.name, Destination: extPacket.Packet.SearchRequest.Origin, HopLimit: uint32(hopLimit), Results: searchResults}
		packetToSend := &GossipPacket{SearchReply: searchReply}

		go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.SearchReply.HopLimit, packetToSend.SearchReply.Destination)
	}

	extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1
	gossiper.forwardSearchRequestWithBudget(extPacket)

}

func (gossiper *Gossiper) forwardSearchRequestWithBudget(extPacket *ExtendedGossipPacket) {

	peers := gossiper.GetPeersAtomic()
	peersChosen := []*net.UDPAddr{extPacket.SenderAddr}
	availablePeers := helpers.DifferenceString(peers, peersChosen)

	if extPacket.Packet.SearchRequest.Budget > 0 && len(availablePeers) != 0 {

		if debug {
			fmt.Println("Handling request from " + extPacket.SenderAddr.String() + " with total budget " + fmt.Sprint(extPacket.Packet.SearchRequest.Budget))
		}

		budgetForEach := extPacket.Packet.SearchRequest.Budget / uint64(len(availablePeers))
		budgetToShare := extPacket.Packet.SearchRequest.Budget % uint64(len(availablePeers))

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
			gossiper.sendPacket(extPacket.Packet, randomPeer)
			peersChosen = append(peersChosen, randomPeer)
			availablePeers = helpers.DifferenceString(gossiper.GetPeersAtomic(), peersChosen)
		}
	}
}

func (gossiper *Gossiper) storeChunksOwner(destination string, chunkMap []uint64, fileMetadata *FileMetadata) {

	metafile := *fileMetadata.MetaFile

	for _, elem := range chunkMap {

		chunkHash := metafile[(elem-1)*32 : elem*32]

		value, loaded := gossiper.fileHandler.myFileChunks.LoadOrStore(hex.EncodeToString(chunkHash), &ChunkOwners{})
		chunkOwner := value.(*ChunkOwners)
		if !loaded {
			chunkOwner.Owners = make([]string, 0)
		}
		chunkOwner.Owners = helpers.RemoveDuplicatesFromSlice(append(chunkOwner.Owners, destination))
	}
}

func (gossiper *Gossiper) addSearchFileForGUI(fileMetadata *FileMetadata) {

	element := FileGUI{Name: fileMetadata.FileName, MetaHash: hex.EncodeToString(fileMetadata.MetafileHash)}
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
