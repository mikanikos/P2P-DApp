package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) indexFile(fileName *string) {

	file, err := os.Open(shareFolder + *fileName)
	helpers.ErrorCheck(err)
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

	if hw3ex2Mode {
		tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
		block := BlockPublish{Transaction: tx}
		go func(b BlockPublish) {
			gossiper.clientBlockBuffer <- b
		}(block)

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
