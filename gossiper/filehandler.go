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
	lastSearchRequests MutexSearchResult
}

// NewFileHandler create new file handler
func NewFileHandler() *FileHandler {
	return &FileHandler{
		myFileChunks:       sync.Map{},
		myFiles:            sync.Map{},
		hashChannels:       sync.Map{},
		filesList:          sync.Map{},
		lastSearchRequests: MutexSearchResult{SearchResults: make(map[string]time.Time)},
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

func initializeDirectories() {
	wd, err := os.Getwd()
	helpers.ErrorCheck(err)

	shareFolder = wd + shareFolder
	downloadFolder = wd + downloadFolder

	os.Mkdir(shareFolder, os.ModePerm)
	os.Mkdir(downloadFolder, os.ModePerm)
}

func (gossiper *Gossiper) indexFile(fileName *string) {

	file, err := os.Open(shareFolder + *fileName)
	helpers.ErrorCheck(err)
	defer file.Close()

	fileInfo, err := file.Stat()
	helpers.ErrorCheck(err)

	fileSize := fileInfo.Size()
	numFileChunks := uint64(math.Ceil(float64(fileSize) / float64(fileChunk)))
	chunkMap := make([]uint64, numFileChunks)
	hashes := make([]byte, numFileChunks*sha256.Size)

	for i := uint64(0); i < numFileChunks; i++ {
		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)
		file.Read(partBuffer)

		hash32 := sha256.Sum256(partBuffer)
		hash := hash32[:]

		gossiper.fileHandler.myFileChunks.Store(hex.EncodeToString(hash), &ChunkOwners{Data: &partBuffer, Owners: make([]string, 0)})
		copy(hashes[i*32:(i+1)*32], hash)

		chunkMap[i] = i + 1
	}

	metahash32 := sha256.Sum256(hashes)
	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	fileMetadata := &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks, MetaFile: &hashes, Size: fileSize}

	gossiper.fileHandler.myFiles.Store(keyHash, fileMetadata)
	gossiper.fileHandler.filesList.LoadOrStore(keyHash+*fileName, &FileIDPair{FileName: *fileName, EncMetaHash: keyHash})

	if hw3ex2Mode || hw3ex3Mode || hw3ex4Mode {
		tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
		block := BlockPublish{Transaction: tx, PrevHash: gossiper.blHandler.previousBlockHash}
		extPacket := gossiper.createTLCMessage(block, -1, rand.Float32())

		if hw3ex2Mode && !hw3ex3Mode && !hw3ex4Mode {
			gossiper.gossipWithConfirmation(extPacket, false)
		} else {
			go func(e *ExtendedGossipPacket) {
				gossiper.blHandler.blockBuffer <- e
			}(extPacket)
		}
	} else {
		go func(f *FileMetadata) {
			gossiper.uiHandler.filesIndexed <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
		}(fileMetadata)
	}

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
		fmt.Println(fileSize)
	}
}

func (gossiper *Gossiper) processClientBlocks() {
	for extPacket := range gossiper.blHandler.blockBuffer {

		if hw3ex4Mode {
			gossiper.qscRound(extPacket)
		} else {
			gossiper.tlcRound(extPacket)
		}
	}
}
