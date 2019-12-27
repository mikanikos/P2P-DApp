package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// FileHandler struct
type FileHandler struct {
	myFileChunks       sync.Map
	myFiles            sync.Map
	hashChannels       sync.Map
	filesList          sync.Map
	lastSearchRequests SafeRequestMap
}

// NewFileHandler create new file handler
func NewFileHandler() *FileHandler {
	return &FileHandler{
		myFileChunks:       sync.Map{},
		myFiles:            sync.Map{},
		hashChannels:       sync.Map{},
		filesList:          sync.Map{},
		lastSearchRequests: SafeRequestMap{OriginTimeMap: make(map[string]time.Time)},
	}
}

// FileMetadata struct
type FileMetadata struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
	MetaFile     *[]byte
	Size         int64
}

// FileIDPair struct
type FileIDPair struct {
	FileName    string
	EncMetaHash string
}

// ChunkOwners struct
type ChunkOwners struct {
	Data   *[]byte
	Owners []string
}

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

	chunkMap := make([]uint64, numFileChunks)
	hashes := make([]byte, numFileChunks*sha256.Size)

	// get and save each chunk of the file
	for i := uint64(0); i < numFileChunks; i++ {
		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)
		file.Read(partBuffer)

		hash32 := sha256.Sum256(partBuffer)
		hash := hash32[:]

		// save chunk
		gossiper.fileHandler.myFileChunks.Store(hex.EncodeToString(hash), &ChunkOwners{Data: &partBuffer, Owners: make([]string, 0)})
		copy(hashes[i*32:(i+1)*32], hash)

		chunkMap[i] = i + 1
	}

	metahash32 := sha256.Sum256(hashes)
	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	// save all file metadata
	fileMetadata := &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks, MetaFile: &hashes, Size: fileSize}

	// save file name
	gossiper.fileHandler.myFiles.Store(keyHash, fileMetadata)
	gossiper.fileHandler.filesList.LoadOrStore(keyHash+*fileName, &FileIDPair{FileName: *fileName, EncMetaHash: keyHash})

	// process file request depending on flags
	if hw3ex2Mode || hw3ex3Mode || hw3ex4Mode {

		// create tx block
		tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
		block := BlockPublish{Transaction: tx, PrevHash: gossiper.blockchainHandler.previousBlockHash}
		extPacket := gossiper.createTLCMessage(block, -1, rand.Float32())

		// if no simple gossip with confirmation, send it to client block buffer
		if hw3ex2Mode && !hw3ex3Mode && !hw3ex4Mode {
			gossiper.gossipWithConfirmation(extPacket, false)
		} else {
			go func(e *ExtendedGossipPacket) {
				gossiper.blockchainHandler.blockBuffer <- e
			}(extPacket)
		}
	} else {
		// if no processing of blocks, just send it to gui
		go func(f *FileMetadata) {
			gossiper.uiHandler.filesIndexed <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
		}(fileMetadata)
	}

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
		fmt.Println(fileSize)
	}
}
