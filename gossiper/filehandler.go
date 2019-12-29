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
	hashDataMap sync.Map

	//myFileChunks       sync.Map
	myFiles      sync.Map
	hashChannels sync.Map
	//filesList          sync.Map
	lastSearchRequests SafeRequestMap
}

// NewFileHandler create new file handler
func NewFileHandler() *FileHandler {
	return &FileHandler{
		hashDataMap: sync.Map{},
		//myFileChunks:       sync.Map{},
		myFiles:      sync.Map{},
		hashChannels: sync.Map{},
		//filesList:          sync.Map{},
		lastSearchRequests: SafeRequestMap{OriginTimeMap: make(map[string]time.Time)},
	}
}

// FileMetadata struct
type FileMetadata struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     *ChunkOwnersMap
	ChunkCount   uint64
	Confirmed    bool
	Size         int64
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

// // ChunkOwners struct
// type ChunkOwners struct {
// 	Data   *[]byte
// 	Owners []string
// }

// create directories for shared and downloaded files at the base directory
func initializeDirectories() {
	wd, err := os.Getwd()
	helpers.ErrorCheck(err)

	shareFolder = wd + shareFolder
	downloadFolder = wd + downloadFolder

	os.Mkdir(shareFolder, os.ModePerm)
	os.Mkdir(downloadFolder, os.ModePerm)
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
	chunkMap := &ChunkOwnersMap{ChunkOwners: make(map[uint64][]string)}

	// get and save each chunk of the file
	for i := uint64(0); i < numFileChunks; i++ {
		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)
		file.Read(partBuffer)

		hash32 := sha256.Sum256(partBuffer)
		hash := hash32[:]

		// save chunk
		gossiper.fileHandler.hashDataMap.LoadOrStore(hex.EncodeToString(hash), &partBuffer)
		chunkMap.ChunkOwners[i+1] = make([]string, 0)
		copy(hashes[i*32:(i+1)*32], hash)
	}

	metahash32 := sha256.Sum256(hashes)
	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	// save all file metadata
	gossiper.fileHandler.hashDataMap.LoadOrStore(keyHash, &hashes)
	metadataStored, loaded := gossiper.fileHandler.myFiles.LoadOrStore(getKeyFromString(keyHash+*fileName), &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks, Size: fileSize})
	fileMetadata := metadataStored.(*FileMetadata)

	// publish tx block and agree with other peers on this block, otherwise save it
	if ((hw3ex2Mode || hw3ex3Mode) && !loaded) || hw3ex4Mode {
		go gossiper.createAndPublishTxBlock(fileMetadata)
	} else {
		if !loaded {
			gossiper.confirmMetafileData(*fileName, metahash)
		}
	}

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
	}
}

// make fileMetadata available for other peers and search results
func (gossiper *Gossiper) confirmMetafileData(filename string, hash []byte) {

	value, loaded := gossiper.fileHandler.myFiles.Load(getKeyFromString(hex.EncodeToString(hash) + filename))

	if !loaded {
		if debug {
			fmt.Println("Error: confirming not existing file metadata")
		}
		return
	}

	fileMetadata := value.(*FileMetadata)

	// confirm metafile, i.e. send it to gui and make it available for others
	fileMetadata.Confirmed = true

	go func(f *FileMetadata) {
		gossiper.uiHandler.filesIndexed <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
	}(fileMetadata)

}

func (fileMetadata *FileMetadata) updateChunkOwner(destination string, seqNum uint64) {
	fileMetadata.ChunkMap.Mutex.Lock()
	owners, loaded := fileMetadata.ChunkMap.ChunkOwners[seqNum]
	if !loaded {
		fileMetadata.ChunkMap.ChunkOwners[seqNum] = make([]string, 0)
		owners = fileMetadata.ChunkMap.ChunkOwners[seqNum]
	}
	fileMetadata.ChunkMap.ChunkOwners[seqNum] = helpers.RemoveDuplicatesFromSlice(append(owners, destination))
	fileMetadata.ChunkMap.Mutex.Unlock()
}

// func (gossiper *Gossiper) saveMetafileData(hash []byte, filename string, data *[]byte) (*FileMetadata, bool) {
// 	gossiper.fileHandler.hashDataMap.Store(hex.EncodeToString(hash), data)
// 	value, loaded := gossiper.fileHandler.myFiles.LoadOrStore(getKeyFromString(hex.EncodeToString(hash)+filename), &FileMetadata{FileName: filename, MetafileHash: hash, ChunkMap: &ChunkOwnersMap{ChunkOwners: make(map[uint64][]string)}, ChunkCount: uint64(len(*data) / 32)})
// 	return value.(*FileMetadata), loaded
// }

func (fileMetadata *FileMetadata) getChunkOwnersByID(seqNum uint64) []string {
	fileMetadata.ChunkMap.Mutex.RLock()
	defer fileMetadata.ChunkMap.Mutex.RUnlock()

	peersWithChunk, loaded := fileMetadata.ChunkMap.ChunkOwners[seqNum]
	if !loaded {
		fileMetadata.ChunkMap.ChunkOwners[seqNum] = make([]string, 0)
		peersWithChunk = fileMetadata.ChunkMap.ChunkOwners[seqNum]
	}
	return peersWithChunk
}

// check if, given a file, I know all the chunks location (at least one peer per chunk)
func (fileMetadata *FileMetadata) checkAllChunksLocation() bool {

	fileMetadata.ChunkMap.Mutex.RLock()
	defer fileMetadata.ChunkMap.Mutex.RUnlock()

	chunkCounter := uint64(0)
	for _, peers := range fileMetadata.ChunkMap.ChunkOwners {
		if len(peers) != 0 {
			chunkCounter++
		}
	}

	if fileMetadata.ChunkCount == chunkCounter {
		return true
	}

	return false
}

func (chunkMap *ChunkOwnersMap) getKeyList() []uint64 {

	chunkMap.Mutex.RLock()
	defer chunkMap.Mutex.RUnlock()

	keys := make([]uint64, len(chunkMap.ChunkOwners))

	i := 0
	for k := range chunkMap.ChunkOwners {
		keys[i] = k
		i++
	}
	return keys
}
