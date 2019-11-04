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
	Name     *string
	MetaFile *[]byte
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

	hashes := make([]byte, numFileChunks*sha256.Size)

	for i := uint64(0); i < numFileChunks; i++ {

		partSize := int(math.Min(fileChunk, float64(fileSize-int64(i*fileChunk))))
		partBuffer := make([]byte, partSize)
		file.Read(partBuffer)

		hash32 := sha256.Sum256(partBuffer)
		hash := hash32[:]

		gossiper.myFileChunks.Store(hex.EncodeToString(hash), &partBuffer)
		copy(hashes[i*32:(i+1)*32], hash)
	}

	metahash32 := sha256.Sum256(hashes)
	fileMetadata := &FileMetadata{Name: fileName, MetaFile: &hashes}

	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	gossiper.mySharedFiles.Store(keyHash, fileMetadata)

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
		fmt.Println(fileSize)
	}
}
