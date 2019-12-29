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
	chunkOwnership     *ChunkOwnersMap

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
		chunkOwnership:     &ChunkOwnersMap{ChunkOwners: make(map[string][]string)},

		filesIndexed:    make(chan *FileGUI, 10),
		filesDownloaded: make(chan *FileGUI, 10),
		filesSearched:   make(chan *FileGUI, 10),
	}
}

// FileMetadata struct
type FileMetadata struct {
	FileName     string
	MetafileHash []byte
	ChunkCount   uint64
	ChunkMap     []uint64
	Size         int64
}

// FileIDPair struct
type FileIDPair struct {
	FileName    string
	EncMetaHash string
}

// ChunkOwnersMap struct
type ChunkOwnersMap struct {
	ChunkOwners map[string][]string
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
	metadataStored, loaded := gossiper.fileHandler.filesMetadata.LoadOrStore(getKeyFromString(keyHash+*fileName), &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks, Size: fileSize})
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
func (fileHandler *FileHandler) updateChunkMap(fileMetadata *FileMetadata, metafilePointer *[]byte) {
	metafile := *metafilePointer
	for i := uint64(0); i < fileMetadata.ChunkCount; i++ {
		hashChunk := metafile[i*32 : (i+1)*32]
		//		fmt.Println("Updating on chunk hash : " + hex.EncodeToString(hashChunk) + " numero " + fmt.Sprint(i+1))
		_, loaded := fileHandler.hashDataMap.Load(hex.EncodeToString(hashChunk))
		if loaded {
			fileMetadata.ChunkMap = helpers.RemoveDuplicatesFromUint64Slice(helpers.InsertToSortUint64Slice(fileMetadata.ChunkMap, i+1))
		}
	}
}

// check if, given a file, I know all the chunks location (at least one peer per chunk)
func (fileHandler *FileHandler) checkAllChunksLocation(fileMetadata *FileMetadata) bool {

	metafileStored, loaded := fileHandler.hashDataMap.Load(hex.EncodeToString(fileMetadata.MetafileHash))
	if !loaded {
		return false
	}

	fileHandler.chunkOwnership.Mutex.RLock()
	defer fileHandler.chunkOwnership.Mutex.RUnlock()

	metafile := *metafileStored.(*[]byte)
	for i := uint64(0); i < fileMetadata.ChunkCount; i++ {
		hash := metafile[i*32 : (i+1)*32]
		chunkValue, loaded := fileHandler.chunkOwnership.ChunkOwners[hex.EncodeToString(hash)]
		if loaded {
			if len(chunkValue) == 0 {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
