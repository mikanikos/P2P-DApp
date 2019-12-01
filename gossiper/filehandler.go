package gossiper

import (
	"encoding/hex"
	"os"
	"strings"
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

func containsKeyword(fileName string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(fileName, keyword) {
			return true
		}
	}
	return false
}

func (gossiper *Gossiper) checkAllChunksLocation(metafile []byte, numChunks uint64) bool {
	for i := uint64(0); i < numChunks; i++ {
		hash := metafile[i*32 : (i+1)*32]
		chunkValue, loaded := gossiper.fileHandler.myFileChunks.Load(hex.EncodeToString(hash))
		if loaded {
			chunkOwnerEntry := chunkValue.(*ChunkOwners)
			if len(chunkOwnerEntry.Owners) == 0 {
				return false
			}
		} else {
			return false
		}
	}
	return true
}
