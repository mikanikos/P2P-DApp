package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

func (gossiper *Gossiper) requestFile(fileName string, packet *GossipPacket) {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)
	value, loaded := gossiper.hashChannels.LoadOrStore(metahashEnc+packet.DataRequest.Destination, make(chan *DataReply))
	metaFileChan := value.(chan *DataReply)

	if loaded {
		// already requesting this file
		return
	}

	value, loaded = gossiper.myDownloadedFiles.Load(metahashEnc)
	if loaded {
		// already have this file

		fileMetadata := value.(*FileMetadata)
		copyFile(*fileMetadata.Name, fileName)
		return
	}

	messageToPrint := "DOWNLOADING metafile of " + fileName + " from " + packet.DataRequest.Destination
	replyPacket := gossiper.requestMetafilePeriodically(packet, metaFileChan, messageToPrint)

	//fmt.Println("Got metafile")

	//fmt.Println(replyPacket.Data)
	chunksHash := getChunksFromMetafile(replyPacket.Data)

	chunksOutChan := make(chan *DataReply)
	numChunksToWait := gossiper.getAllChunks(fileName, replyPacket.Origin, chunksHash, chunksOutChan)

	//numChunksHash := len(chunksHash)

	//fmt.Println(numChunksHash)

	// chunksOutChan := make(chan *DataReply)
	// var wg sync.WaitGroup
	// seqNum := 1
	// numChunksToWait := 0

	// for _, hashChunk := range chunksHash {

	// 	key := hex.EncodeToString(hashChunk)

	// 	_, loaded := gossiper.myFileChunks.Load(key)
	// 	if !loaded {

	// 		value, loaded := gossiper.hashChannels.LoadOrStore(key, make(chan *DataReply))
	// 		if !loaded {

	// 			numChunksToWait++
	// 			chunkIn := value.(chan *DataReply)
	// 			wg.Add(1)

	// 			dataRequest := &DataRequest{Origin: gossiper.name, Destination: replyPacket.Origin, HashValue: hashChunk, HopLimit: uint32(hopLimit)}
	// 			newPacket := &GossipPacket{DataRequest: dataRequest}

	// 			messageToPrint = "DOWNLOADING " + fileName + " chunk " + fmt.Sprint(seqNum) + " from " + packet.DataRequest.Destination
	// 			go gossiper.requestChunkPeriodically(newPacket, chunkIn, chunksOutChan, &wg, messageToPrint)
	// 		}
	// 	}
	// 	seqNum++
	// }

	// fmt.Println("Waiting for all finishing")
	// wg.Wait()

	// fmt.Println("Got all chunks")
	// chunksData := make([]byte, numChunksHash*fileChunk)

	// count := 0
	// size := 0
	// emptyAnswer := false

	// for chunkPacket := range chunksOutChan {
	// 	for i, hChunk := range chunksHash {
	// 		if hex.EncodeToString(chunkPacket.HashValue) == hex.EncodeToString(hChunk) {
	// 			chunkLen := len(chunkPacket.Data)
	// 			if chunkLen != 0 {
	// 				copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunkPacket.Data)
	// 				gossiper.myFileChunks.Store(hex.EncodeToString(chunkPacket.HashValue), chunkPacket.Data)
	// 				size += chunkLen
	// 			} else {
	// 				emptyAnswer = true
	// 			}
	// 			break
	// 		}
	// 	}
	// 	count++
	// 	if count == numChunksHash {
	// 		break
	// 	}
	// }

	containsEmpty := false
	if numChunksToWait != 0 {
		containsEmpty = gossiper.processIncomingChunkData(numChunksToWait, chunksOutChan)
	}

	if !containsEmpty {

		chunksData, size := gossiper.retrieveChunks(chunksHash)

		fileReconstructed := make([]byte, size)
		copy(fileReconstructed, chunksData[:size])

		//fmt.Println(len(fileReconstructed))

		// for chunkPacket := range chunksOutChan {
		// 	for i, h := range chunksHash {
		// 		var hash32 [32]byte
		// 		copy(hash32[:], chunkPacket.HashValue)
		// 		if hash32 == h {
		// 			chunksHash[i] = chunksHash[len(chunksHash)-1]
		// 			chunksHash = chunksHash[:len(chunksHash)-1]
		// 			chunksData = append(chunksData, chunkPacket.Data...)
		// 			gossiper.myFileChunks.Store(h, chunkPacket.Data)
		// 		}
		// 	}
		// 	if len(chunksHash) == 0 {
		// 		break
		// 	}
		// }

		reconstructFileFromChunks(fileName, fileReconstructed)
		fmt.Println("RECONSTRUCTED file " + fileName)

		fileMetadata := &FileMetadata{Name: &fileName, MetaFile: &replyPacket.Data}

		gossiper.myDownloadedFiles.Store(metahashEnc, fileMetadata)
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

	fmt.Println("Waiting for all finishing")
	wg.Wait()

	fmt.Println("Got all chunks")

	return numChunksToWait
}

func (gossiper *Gossiper) processIncomingChunkData(numChunksToWait int, chunksOutChan chan *DataReply) bool {
	containsEmpty := false

	// for chunkPacket := range chunksOutChan {
	// 	for i, hChunk := range chunksHash {
	// 		if hex.EncodeToString(chunkPacket.HashValue) == hex.EncodeToString(hChunk) {
	// 			chunkLen := len(chunkPacket.Data)
	// 			if chunkLen != 0 {
	// 				copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunkPacket.Data)
	// 				gossiper.myFileChunks.Store(hex.EncodeToString(chunkPacket.HashValue), chunkPacket.Data)
	// 				size += chunkLen
	// 			} else {
	// 				containsEmpty = true
	// 			}
	// 			break
	// 		}
	// 	}
	// 	count++
	// 	if count == numChunksHash {
	// 		break
	// 	}
	// }

	count := 0

	for chunkPacket := range chunksOutChan {
		if len(chunkPacket.Data) != 0 {
			gossiper.myFileChunks.Store(hex.EncodeToString(chunkPacket.HashValue), chunkPacket.Data)
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
		chunk := value.([]byte)
		chunkLen := len(chunk)
		copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunk)
		size += chunkLen
	}

	return chunksData, size
}

func (gossiper *Gossiper) requestMetafilePeriodically(packet *GossipPacket, metaFileChan chan *DataReply, messageToPrint string) *DataReply {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)

	fmt.Println(messageToPrint)
	go gossiper.forwardDataRequest(packet)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	fmt.Println("Waiting for metafile....")
	for {
		select {
		case replyPacket := <-metaFileChan:
			timer.Stop()
			close(metaFileChan)
			gossiper.hashChannels.Delete(metahashEnc + replyPacket.Origin)
			return replyPacket

		case <-timer.C:
			fmt.Println(messageToPrint)
			go gossiper.forwardDataRequest(packet)
		}
	}
}

func (gossiper *Gossiper) requestChunkPeriodically(newPacket *GossipPacket, chunkIn chan *DataReply, chanOut chan *DataReply, wg *sync.WaitGroup, messageToPrint string) {

	defer wg.Done()

	// chunkHashEnc := hex.EncodeToString(newPacket.DataRequest.HashValue)
	// value, _ := gossiper.hashChannels.LoadOrStore(chunkHashEnc, make(chan *DataReply))
	// chunkIn := value.(chan *DataReply)

	fmt.Println(messageToPrint)
	go gossiper.forwardDataRequest(newPacket)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-chunkIn:
			fmt.Println("Got chunk")
			timer.Stop()
			fmt.Println("Closing channel")
			close(chunkIn)
			gossiper.hashChannels.Delete(hex.EncodeToString(newPacket.DataRequest.HashValue) + replyPacket.Origin)
			fmt.Println("Sending to output channel")
			go func(r *DataReply) {
				chanOut <- r
			}(replyPacket)
			fmt.Println("Done")
			return

		case <-timer.C:
			fmt.Println(messageToPrint)
			go gossiper.forwardDataRequest(newPacket)
		}
	}
}
