package gossiper

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) downloadMetafile(destination string, fileMetadata *FileMetadata) {

	metaHashEnc := hex.EncodeToString(fileMetadata.MetafileHash)
	value, loaded := gossiper.hashChannels.LoadOrStore(metaHashEnc+destination, make(chan *DataReply))
	metaFileChan := value.(chan *DataReply)

	if loaded {
		if debug {
			fmt.Println("Already requesting this metafile")
		}
		return
	}

	dataRequest := &DataRequest{Origin: gossiper.name, Destination: destination, HashValue: fileMetadata.MetafileHash, HopLimit: uint32(hopLimit)}
	packet := &GossipPacket{DataRequest: dataRequest}

	if hw2 {
		fmt.Println("DOWNLOADING metafile of " + fileMetadata.FileName + " from " + destination)
	}
	go gossiper.forwardPrivateMessage(packet, &packet.DataRequest.HopLimit, packet.DataRequest.Destination)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-metaFileChan:

			if replyPacket.Data != nil && checkHash(fileMetadata.MetafileHash, replyPacket.Data) {

				timer.Stop()
				close(metaFileChan)
				gossiper.hashChannels.Delete(metaHashEnc + destination)

				fileMetadata.MetaFile = &replyPacket.Data
				fileMetadata.ChunkCount = uint64(len(replyPacket.Data) / 32)

				if debug {
					fmt.Println("Got metafile")
				}
				return
			}

		case <-timer.C:
			if hw2 {
				fmt.Println("DOWNLOADING metafile of " + fileMetadata.FileName + " from " + destination)
			}
			go gossiper.forwardPrivateMessage(packet, &packet.DataRequest.HopLimit, packet.DataRequest.Destination)
		}
	}
}

func (gossiper *Gossiper) requestFile(fileName string, metaHash []byte, destination string) {

	value, loaded := gossiper.myFiles.Load(hex.EncodeToString(metaHash))

	if !loaded {
		if destination == "" {
			if debug {
				fmt.Println("ERROR: file not found")
			}
			return
		}
		value, _ = gossiper.myFiles.LoadOrStore(hex.EncodeToString(metaHash), &FileMetadata{FileName: fileName, MetafileHash: metaHash, ChunkMap: make([]uint64, 0)})
		gossiper.filesList.LoadOrStore(hex.EncodeToString(metaHash)+fileName, &FileIDPair{FileName: fileName, EncMetaHash: hex.EncodeToString(metaHash)})
		fileMetadata := value.(*FileMetadata)
		gossiper.downloadMetafile(destination, fileMetadata)
	}

	fileMetadata := value.(*FileMetadata)

	if fileMetadata.Size != 0 {
		if debug {
			fmt.Println("Already have this file")
		}
		if fileMetadata.FileName != fileName {
			if debug {
				fmt.Println("Same file content but different name, just copy it")
			}
			copyFile(fileMetadata.FileName, fileName)
		}
		return
	}

	metafile := *fileMetadata.MetaFile

	for i := uint64(0); i < fileMetadata.ChunkCount; i++ {
		hashChunk := metafile[i*32 : (i+1)*32]
		chunkLoaded, _ := gossiper.myFileChunks.LoadOrStore(hex.EncodeToString(hashChunk), &ChunkOwners{})
		chunkOwner := chunkLoaded.(*ChunkOwners)
		gossiper.downloadChunk(fileMetadata, chunkOwner, uint64(i+1), destination)
	}

	if debug {
		fmt.Println("Got all chunks")
	}

	if chunksIntegrityCheck(fileMetadata) {
		gossiper.reconstructFileFromChunks(fileMetadata)

		if hw2 {
			fmt.Println("RECONSTRUCTED file " + fileName)
		}

		go func(f *FileMetadata) {
			gossiper.filesDownloaded <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}

		}(fileMetadata)
	}
}

func (gossiper *Gossiper) downloadChunk(fileMetadata *FileMetadata, chunkOwner *ChunkOwners, seqNum uint64, destination string) {

	metafile := *fileMetadata.MetaFile
	hashChunk := metafile[(seqNum-1)*32 : (seqNum)*32]

	hashChunkEnc := hex.EncodeToString(hashChunk)
	peersWithChunk := chunkOwner.Owners

	if destination != "" {
		peersWithChunk = append(peersWithChunk, destination)
	}

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
			fmt.Println("DOWNLOADING " + fileMetadata.FileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination)
		}
		go gossiper.forwardPrivateMessage(newPacket, &newPacket.DataRequest.HopLimit, newPacket.DataRequest.Destination)

		timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
		for {
			select {
			case replyPacket := <-chunkIn:
				if debug {
					fmt.Println("Got chunk")
				}
				if replyPacket.Data != nil && checkHash(replyPacket.HashValue, replyPacket.Data) {

					timer.Stop()
					close(chunkIn)
					gossiper.hashChannels.Delete(hex.EncodeToString(newPacket.DataRequest.HashValue) + replyPacket.Origin)

					chunkOwner.Data = &replyPacket.Data
					if destination != "" {
						chunkOwner.Owners = helpers.RemoveDuplicatesFromSlice(append(chunkOwner.Owners, destination))
					}
					fileMetadata.ChunkMap = insertToSortUint64Slice(fileMetadata.ChunkMap, seqNum)

					return
				}

			case <-timer.C:
				if hw2 {
					fmt.Println("DOWNLOADING " + fileMetadata.FileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination)
				}
				go gossiper.forwardPrivateMessage(newPacket, &newPacket.DataRequest.HopLimit, newPacket.DataRequest.Destination)
			}
		}
	}
}

func (gossiper *Gossiper) reconstructFileFromChunks(fileMetadata *FileMetadata) {
	size := int64(0)
	chunksData := make([]byte, fileMetadata.ChunkCount*fileChunk)
	metafile := *fileMetadata.MetaFile

	for i := 0; i < int(fileMetadata.ChunkCount); i++ {
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
		size += int64(chunkLen)
	}

	fileReconstructed := make([]byte, size)
	copy(fileReconstructed, chunksData[:size])

	fileMetadata.Size = size

	saveFileOnDisk(fileMetadata.FileName, fileReconstructed)
}
