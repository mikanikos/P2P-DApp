package gossiper

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) downloadMetafile(destination string, fileMetadata *FileMetadata) {

	metaHashEnc := hex.EncodeToString(fileMetadata.FileSearchData.MetafileHash)
	value, loaded := gossiper.hashChannels.LoadOrStore(metaHashEnc+destination, make(chan *DataReply))
	metaFileChan := value.(chan *DataReply)

	if loaded {
		if debug {
			fmt.Println("Already requesting this metafile")
		}
		return
	}

	dataRequest := &DataRequest{Origin: gossiper.name, Destination: destination, HashValue: fileMetadata.FileSearchData.MetafileHash, HopLimit: uint32(hopLimit)}
	packet := &GossipPacket{DataRequest: dataRequest}

	if hw2 {
		fmt.Println("DOWNLOADING metafile of " + fileMetadata.FileSearchData.FileName + " from " + destination)
	}
	go gossiper.forwardPrivateMessage(packet)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-metaFileChan:
			timer.Stop()
			close(metaFileChan)
			gossiper.hashChannels.Delete(metaHashEnc + destination)

			if !checkHash(fileMetadata.FileSearchData.MetafileHash, replyPacket.Data) {
				fmt.Println("ERROR: metafile doesn't correspond to hash")
			} else {
				fileMetadata.MetaFile = &replyPacket.Data
			}

			if debug {
				fmt.Println("Got metafile")
			}
			return

		case <-timer.C:
			if hw2 {
				fmt.Println("DOWNLOADING metafile of " + fileMetadata.FileSearchData.FileName + " from " + destination)
			}
			go gossiper.forwardPrivateMessage(packet)
		}
	}
}

func (gossiper *Gossiper) requestFile(fileName string, metaHash []byte) {

	value, loaded := gossiper.myFiles.Load(hex.EncodeToString(metaHash))

	if !loaded {
		if debug {
			fmt.Println("ERROR: file not found")
		}
		return
	}

	fileMetadata := value.(*FileMetadata)

	if fileMetadata.Size != 0 {
		if debug {
			fmt.Println("Already have this file")
		}
		if fileMetadata.FileSearchData.FileName != fileName {
			if debug {
				fmt.Println("Same file content but different name, just copy it")
			}
			copyFile(fileMetadata.FileSearchData.FileName, fileName)
		}
		return
	}

	metafile := *fileMetadata.MetaFile

	var wg sync.WaitGroup
	wg.Add(int(fileMetadata.FileSearchData.ChunkCount))

	for i := uint64(0); i < fileMetadata.FileSearchData.ChunkCount; i++ {

		hashChunk := metafile[i*32 : (i+1)*32]

		value, loaded := gossiper.myFileChunks.Load(hex.EncodeToString(hashChunk))
		if !loaded {
			if debug {
				fmt.Println("ERROR: no chunk location found")
			}
			return
		}

		chunkOwner := value.(*ChunkOwners)

		go gossiper.downloadChunk(fileMetadata, chunkOwner, hashChunk, &wg, uint64(i+1))
	}

	wg.Wait()

	if debug {
		fmt.Println("Got all chunks")
	}

	// check
	for i := uint64(0); i < fileMetadata.FileSearchData.ChunkCount; i++ {
		if fileMetadata.FileSearchData.ChunkMap[i] != i+1 {
			if debug {
				fmt.Println("ERROR: not all chunks retrieved")
			}
			return
		}
	}

	if fileMetadata.FileSearchData.ChunkCount == uint64(len(fileMetadata.FileSearchData.ChunkMap)) {
		gossiper.reconstructFileFromChunks(fileMetadata)

		if hw2 {
			fmt.Println("RECONSTRUCTED file " + fileName)
		}

		go func(f *FileMetadata) {
			gossiper.filesDownloaded <- f
		}(fileMetadata)
	}
}

func (gossiper *Gossiper) downloadChunk(fileMetadata *FileMetadata, chunkOwner *ChunkOwners, hashChunk []byte, wg *sync.WaitGroup, seqNum uint64) {

	hashChunkEnc := hex.EncodeToString(hashChunk)
	peersWithChunk := chunkOwner.Owners
	defer wg.Done()

	// it is possible to check if the chunk id is present in the chunkMap of the fileMetadata (maybe better?) but this is easier to do
	for chunkOwner.Data == nil && len(peersWithChunk) != 0 {

		peersLength := len(peersWithChunk)
		randIndex := rand.Intn(peersLength)
		destination := peersWithChunk[randIndex]
		peersWithChunk[randIndex] = peersWithChunk[peersLength-1]
		peersWithChunk = peersWithChunk[:peersLength-1]

		value, loaded := gossiper.hashChannels.LoadOrStore(hashChunkEnc+destination, make(chan *DataReply))
		if loaded {
			if debug {
				fmt.Println("Already requesting this chunk")
			}
			return
		}

		chunkIn := value.(chan *DataReply)
		dataRequest := &DataRequest{Origin: gossiper.name, Destination: destination, HashValue: hashChunk, HopLimit: uint32(hopLimit)}
		newPacket := &GossipPacket{DataRequest: dataRequest}

		if hw2 {
			fmt.Println("DOWNLOADING " + fileMetadata.FileSearchData.FileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination)
		}
		go gossiper.forwardPrivateMessage(newPacket)

		timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
		keepWaiting := true
		for keepWaiting {
			select {
			case replyPacket := <-chunkIn:
				if debug {
					fmt.Println("Got chunk")
				}
				timer.Stop()
				close(chunkIn)
				gossiper.hashChannels.Delete(hex.EncodeToString(newPacket.DataRequest.HashValue) + replyPacket.Origin)
				if len(replyPacket.Data) != 0 && checkHash(replyPacket.HashValue, replyPacket.Data) {
					chunkOwner.Data = &replyPacket.Data
					fileMetadata.FileSearchData.ChunkMap = insertToSortUint64Slice(fileMetadata.FileSearchData.ChunkMap, seqNum)
				}
				keepWaiting = false

			case <-timer.C:
				if hw2 {
					fmt.Println("DOWNLOADING " + fileMetadata.FileSearchData.FileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination)
				}
				go gossiper.forwardPrivateMessage(newPacket)
			}
		}
	}
}

func (gossiper *Gossiper) reconstructFileFromChunks(fileMetadata *FileMetadata) {
	size := 0
	chunksData := make([]byte, fileMetadata.FileSearchData.ChunkCount*fileChunk)
	metafile := *fileMetadata.MetaFile

	for i := 0; i < int(fileMetadata.FileSearchData.ChunkCount); i++ {
		hChunk := metafile[i*32 : (i+1)*32]
		value, loaded := gossiper.myFileChunks.Load(hex.EncodeToString(hChunk))

		if !loaded {
			if debug {
				fmt.Println("ERROR: no chunk during reconstruction")
			}
			return
		}

		chunkOwner := value.(*ChunkOwners)
		chunk := *chunkOwner.Data
		chunkLen := len(chunk)
		copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunk)
		size += chunkLen
	}

	fileReconstructed := make([]byte, size)
	copy(fileReconstructed, chunksData[:size])

	fileMetadata.Size = size

	wd, err := os.Getwd()
	helpers.ErrorCheck(err)

	fullPath := wd + downloadFolder + fileMetadata.FileSearchData.FileName
	file, err := os.Create(fullPath)
	helpers.ErrorCheck(err)

	defer file.Close()

	_, err = file.Write(fileReconstructed)
	helpers.ErrorCheck(err)

	err = file.Sync()
	helpers.ErrorCheck(err)
}

// GetFilesDownloaded for GUI
func (gossiper *Gossiper) GetFilesDownloaded() []FileMetadata {

	bufferLength := len(gossiper.filesDownloaded)

	files := make([]FileMetadata, bufferLength)
	for i := 0; i < bufferLength; i++ {
		file := <-gossiper.filesDownloaded
		files[i] = *file
	}

	return files
}
