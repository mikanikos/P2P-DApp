package gossiper

import (
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) requestFile(fileName string, packet *GossipPacket) {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)
	value, loaded := gossiper.hashChannels.LoadOrStore(metahashEnc+packet.DataRequest.Destination, make(chan *DataReply))
	metaFileChan := value.(chan *DataReply)

	if loaded {
		// already requesting this file
		return
	}

	if loaded {
		// already have this file

		fileMetadata := value.(*FileMetadata)
		wd, err := os.Getwd()
		helpers.ErrorCheck(err)
		copyFile(wd+downloadFolder+fileMetadata.Name, wd+downloadFolder+fileName)
		return
	}

	messageToPrint := "DOWNLOADING metafile of " + fileName + " from " + packet.DataRequest.Destination
	replyPacket := gossiper.requestMetafilePeriodically(packet, metaFileChan, messageToPrint)

	if debug {
		fmt.Println("Got metafile")
	}

	chunksHash := getChunksFromMetafile(replyPacket.Data)

	chunksOutChan := make(chan *DataReply)
	numChunksToWait := gossiper.getAllChunks(fileName, replyPacket.Origin, chunksHash, chunksOutChan)

	containsEmpty := false
	if numChunksToWait != 0 {
		containsEmpty = gossiper.processIncomingChunkData(numChunksToWait, chunksOutChan)
	}

	if !containsEmpty {
		chunksData, size := gossiper.retrieveChunks(chunksHash)

		fileReconstructed := make([]byte, size)
		copy(fileReconstructed, chunksData[:size])

		reconstructFileFromChunks(fileName, fileReconstructed)

		if hw2 {
			fmt.Println("RECONSTRUCTED file " + fileName)
		}

		// TO CHANGEEE
		fileMetadata := &FileMetadata{Name: fileName, MetaFile: &replyPacket.Data, Size: size, MetaHash: metahashEnc}

		go func(f *FileMetadata) {
			gossiper.filesDownloaded <- f
		}(fileMetadata)

		//gossiper.myFiles.Store(metahashEnc, fileMetadata)
	}
}

func (gossiper *Gossiper) getAllChunks(fileName, destination string, chunksHash [][]byte, chunksOutChan chan *DataReply) int {
	var wg sync.WaitGroup
	seqNum := 1
	numChunksToWait := 0

	for _, hashChunk := range chunksHash {

		key := hex.EncodeToString(hashChunk)

		_, loaded := gossiper.myFileChunks.Load(key)
		if !loaded {

			value, loaded := gossiper.hashChannels.LoadOrStore(key+destination, make(chan *DataReply))
			if !loaded {

				numChunksToWait++
				chunkIn := value.(chan *DataReply)
				wg.Add(1)

				dataRequest := &DataRequest{Origin: gossiper.name, Destination: destination, HashValue: hashChunk, HopLimit: uint32(hopLimit)}
				newPacket := &GossipPacket{DataRequest: dataRequest}

				messageToPrint := "DOWNLOADING " + fileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination
				go gossiper.requestChunkPeriodically(newPacket, chunkIn, chunksOutChan, &wg, messageToPrint)
			}
		}
		seqNum++
	}

	if debug {
		fmt.Println("Waiting for all finishing")
	}

	wg.Wait()

	if debug {
		fmt.Println("Got all chunks")
	}

	return numChunksToWait
}

func (gossiper *Gossiper) processIncomingChunkData(numChunksToWait int, chunksOutChan chan *DataReply) bool {
	containsEmpty := false
	count := 0

	for chunkPacket := range chunksOutChan {
		if len(chunkPacket.Data) != 0 {
			gossiper.myFileChunks.Store(hex.EncodeToString(chunkPacket.HashValue), &chunkPacket.Data)
		} else {
			containsEmpty = true
		}
		count++
		if count == numChunksToWait {
			break
		}
	}

	return containsEmpty
}

func (gossiper *Gossiper) retrieveChunks(chunksHash [][]byte) ([]byte, int) {
	size := 0
	chunksData := make([]byte, len(chunksHash)*fileChunk)

	for i, hChunk := range chunksHash {
		value, _ := gossiper.myFileChunks.Load(hex.EncodeToString(hChunk))
		chunk := *value.(*[]byte)
		chunkLen := len(chunk)
		copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunk)
		size += chunkLen
	}

	return chunksData, size
}

func (gossiper *Gossiper) requestMetafilePeriodically(packet *GossipPacket, metaFileChan chan *DataReply, messageToPrint string) *DataReply {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)

	if hw2 {
		fmt.Println(messageToPrint)
	}

	go gossiper.forwardPrivateMessage(packet)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-metaFileChan:
			timer.Stop()
			close(metaFileChan)
			gossiper.hashChannels.Delete(metahashEnc + replyPacket.Origin)
			return replyPacket

		case <-timer.C:
			if hw2 {
				fmt.Println(messageToPrint)
			}

			go gossiper.forwardPrivateMessage(packet)
		}
	}
}

func (gossiper *Gossiper) requestChunkPeriodically(newPacket *GossipPacket, chunkIn chan *DataReply, chanOut chan *DataReply, wg *sync.WaitGroup, messageToPrint string) {

	defer wg.Done()

	if hw2 {
		fmt.Println(messageToPrint)
	}

	go gossiper.forwardPrivateMessage(newPacket)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-chunkIn:
			if debug {
				fmt.Println("Got chunk")
			}

			timer.Stop()
			close(chunkIn)
			gossiper.hashChannels.Delete(hex.EncodeToString(newPacket.DataRequest.HashValue) + replyPacket.Origin)
			go func(r *DataReply) {
				chanOut <- r
			}(replyPacket)
			return

		case <-timer.C:
			if hw2 {
				fmt.Println(messageToPrint)
			}

			go gossiper.forwardPrivateMessage(newPacket)
		}
	}
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
