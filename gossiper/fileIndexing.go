package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sync/atomic"

	"github.com/mikanikos/Peerster/helpers"
)

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

		gossiper.myFileChunks.Store(hex.EncodeToString(hash), &ChunkOwners{Data: &partBuffer, Owners: make([]string, 0)})
		copy(hashes[i*32:(i+1)*32], hash)

		chunkMap[i] = i + 1
	}

	metahash32 := sha256.Sum256(hashes)
	metahash := metahash32[:]
	keyHash := hex.EncodeToString(metahash)

	fileMetadata := &FileMetadata{FileName: *fileName, MetafileHash: metahash, ChunkMap: chunkMap, ChunkCount: numFileChunks, MetaFile: &hashes, Size: fileSize}

	gossiper.myFiles.Store(keyHash, fileMetadata)
	gossiper.filesList.LoadOrStore(keyHash+*fileName, &FileIDPair{FileName: *fileName, EncMetaHash: keyHash})

	go func(f *FileMetadata) {
		gossiper.filesIndexed <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
	}(fileMetadata)

	// tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
	// block := BlockPublish{Transaction: tx}
	// go gossiper.mongerTLCMessage(block)

	if debug {
		fmt.Println("File " + *fileName + " indexed: " + keyHash)
		fmt.Println(fileSize)
	}
}

func (gossiper *Gossiper) mongerTLCMessage(block BlockPublish) {

	id := atomic.LoadUint32(&gossiper.tlcID)
	atomic.AddUint32(&gossiper.tlcID, uint32(1))

	tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.gossiperData.Addr}

	// save message !!!!!
	//gossiper.addMessage(extPacket)

	go gossiper.startRumorMongering(extPacket)

}
