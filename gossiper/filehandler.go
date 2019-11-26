package gossiper

import (
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
