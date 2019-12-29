package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// FileHandler struct
type FileHandler struct {
	hashDataMap        sync.Map
	filesMetadata      sync.Map
	hashChannels       sync.Map
	lastSearchRequests *SafeRequestMap

	filesIndexed    chan *FileGUI
	filesDownloaded chan *FileGUI
	filesSearched   chan *FileGUI
}

// NewFileHandler create new file handler
func NewFileHandler() *FileHandler {
	return &FileHandler{
		hashDataMap:        sync.Map{},
		filesMetadata:      sync.Map{},
		hashChannels:       sync.Map{},
		lastSearchRequests: &SafeRequestMap{OriginTimeMap: make(map[string]time.Time)},

		filesIndexed:    make(chan *FileGUI, 10),
		filesDownloaded: make(chan *FileGUI, 10),
		filesSearched:   make(chan *FileGUI, 10),
	}
}

// FileMetadata struct
type FileMetadata struct {
	FileName       string
	MetafileHash   []byte
	ChunkCount     uint64
	ChunkOwnership *ChunkOwnersMap
	ChunkMap       []uint64
	Size           int64
}

// FileIDPair struct
type FileIDPair struct {
	FileName    string
	EncMetaHash string
}

// ChunkOwnersMap struct
type ChunkOwnersMap struct {
	ChunkOwners map[uint64][]string
	Mutex       sync.RWMutex
}

// index file request from client
func (gossiper *Gossiper) indexFile(fileName *string) {

	// open new file
	file, err := os.Open(shareFolder + *fileName)
	helpers.ErrorCheck(err)
	defer file.Close()

	// get file data
	fileInfo, err := file.Stat()
	helpers.ErrorCheck(err)

	// compute size of the file and number of chunks
	fileSize := fileInfo.Size()
	numFileChunks := uint64(math.Ceil(float64(fileSize) / float64(fileChunk)))
	hashes := make([]byte, numFileChunks*sha256.Size)
	chunkMap := make([]uint64, numFileChunks)

	// get and save each chunk of the file
	for i := uint64(0); i < numFileChunks; i++ {
		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)
		file.Read(partBuffer)

		hash32 := sha256.Sum256(partBuffer)
		hash := hash32[:]

		// save chunk data
		gossiper.fileHandler.hashDataMap.LoadOrStore(hex.EncodeToString(hash), &partBuffer)
		chunkMap[i] = i + 1
		copy(hashes[i*32:(i+1)*32], hash)
	}

	metahash32 := sha256.Sum256(hashes)
	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	// save all file metadata
	gossiper.fileHandler.hashDataMap.LoadOrStore(keyHash, &hashes)
	metadataStored, loaded := gossiper.fileHandler.filesMetadata.LoadOrStore(getKeyFromString(keyHash+*fileName), &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkOwnership: &ChunkOwnersMap{ChunkOwners: make(map[uint64][]string)}, ChunkCount: numFileChunks, Size: fileSize})
	fileMetadata := metadataStored.(*FileMetadata)

	// publish tx block and agree with other peers on this block, otherwise save it and send it to gui
	if ((hw3ex2Mode || hw3ex3Mode) && !loaded) || hw3ex4Mode {
		go gossiper.createAndPublishTxBlock(fileMetadata)
	} else {
		if !loaded {
			go func(f *FileMetadata) {
				gossiper.fileHandler.filesIndexed <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
			}(fileMetadata)
		}
	}

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
	}
}

// update chunk owners map for given file metadata
func (fileMetadata *FileMetadata) updateChunkOwnerMap(destination string, seqNum uint64) {
	fileMetadata.ChunkOwnership.Mutex.Lock()
	defer fileMetadata.ChunkOwnership.Mutex.Unlock()

	owners, loaded := fileMetadata.ChunkOwnership.ChunkOwners[seqNum]
	if !loaded {
		fileMetadata.ChunkOwnership.ChunkOwners[seqNum] = make([]string, 0)
		owners = fileMetadata.ChunkOwnership.ChunkOwners[seqNum]
	}
	fileMetadata.ChunkOwnership.ChunkOwners[seqNum] = helpers.RemoveDuplicatesFromSlice(append(owners, destination))
}

// check if, given a file, I know all the chunks location (at least one peer per chunk)
func (fileMetadata *FileMetadata) checkAllChunksLocation() bool {

	fileMetadata.ChunkOwnership.Mutex.RLock()
	defer fileMetadata.ChunkOwnership.Mutex.RUnlock()

	chunkCounter := uint64(0)
	for _, peers := range fileMetadata.ChunkOwnership.ChunkOwners {
		if len(peers) != 0 {
			chunkCounter++
		}
	}

	if fileMetadata.ChunkCount == chunkCounter {
		return true
	}

	return false
}
