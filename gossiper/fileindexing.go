package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"

	"github.com/mikanikos/Peerster/helpers"
)

// FileMetadata struct
type FileMetadata struct {
	FileSearchData *SearchResult
	MetaFile       *[]byte
	Size           int
}

// ChunkOwners struct
type ChunkOwners struct {
	Data   *[]byte
	Owners []string
}

func initializeDirectories() {
	wd, err := os.Getwd()

	helpers.ErrorCheck(err)

	os.Mkdir(wd+shareFolder, os.ModePerm)
	os.Mkdir(wd+downloadFolder, os.ModePerm)
}

func (gossiper *Gossiper) indexFile(fileName *string) {

	wd, err := os.Getwd()
	helpers.ErrorCheck(err)

	fullPath := wd + shareFolder + *fileName
	file, err := os.Open(fullPath)

	if err != nil {
		if debug {
			fmt.Println("ERROR: No such file " + fullPath)
		}
		return
	}

	defer file.Close()

	fileInfo, _ := file.Stat()
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

		gossiper.myFileChunks.Store(hex.EncodeToString(hash), &partBuffer)
		copy(hashes[i*32:(i+1)*32], hash)

		chunkMap = append(chunkMap, i+1)
	}

	metahash32 := sha256.Sum256(hashes)

	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	searchResult := &SearchResult{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks}
	fileMetadata := &FileMetadata{FileSearchData: searchResult, MetaFile: &hashes, Size: int(fileSize)}

	gossiper.myFiles.Store(keyHash, fileMetadata)

	go func(f *FileMetadata) {
		gossiper.filesIndexed <- f
	}(fileMetadata)

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
		fmt.Println(fileSize)
	}
}

// GetFilesIndexed for GUI
func (gossiper *Gossiper) GetFilesIndexed() []FileMetadata {

	bufferLength := len(gossiper.filesIndexed)

	files := make([]FileMetadata, bufferLength)
	for i := 0; i < bufferLength; i++ {
		file := <-gossiper.filesIndexed
		files[i] = *file
	}

	return files
}

func (gossiper *Gossiper) storeChunksOwner(destination string, chunkMap []uint64, fileMetadata *FileMetadata) {

	for _, elem := range chunkMap {

		metafile := *fileMetadata.MetaFile
		chunkHash := metafile[(elem-1)*32 : elem*32]

		value, loaded := gossiper.myFileChunks.LoadOrStore(hex.EncodeToString(chunkHash), &ChunkOwners{})
		chunkOwner := value.(*ChunkOwners)
		if !loaded {
			chunkOwner.Owners = make([]string, 0)
		}
		chunkOwner.Owners = append(chunkOwner.Owners, destination)
	}
}
