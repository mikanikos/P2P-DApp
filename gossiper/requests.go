package gossiper

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// DataRequest struct
type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

// DataReply struct
type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

func (gossiper *Gossiper) requestFile(fileName string, packet *GossipPacket) {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)
	value, loaded := gossiper.hashChannels.LoadOrStore(metahashEnc, make(chan *DataReply))
	metaFileChan := value.(chan *DataReply)

	if loaded {
		// already requesting this file
		return
	}

	fmt.Println("DOWNLOADING metafile of " + fileName + " from " + packet.DataRequest.Destination)
	replyPacket := gossiper.requestMetafilePeriodically(packet, metaFileChan)

	fmt.Println("Got metafile")

	//fmt.Println(replyPacket.Data)
	chunksHash := getChunksFromMetafile(replyPacket.Data)
	numChunksHash := len(chunksHash)

	fmt.Println(numChunksHash)

	chunksOutChan := make(chan *DataReply)
	var wg sync.WaitGroup

	seqNum := 1
	for _, hashChunk := range chunksHash {

		fmt.Println("DOWNLOADING " + fileName + " chunk " + fmt.Sprint(seqNum) + " from " + packet.DataRequest.Destination)
		wg.Add(1)
		go gossiper.requestChunkPeriodically(hashChunk, replyPacket.Origin, chunksOutChan, &wg)
		seqNum++
	}

	fmt.Println("Waiting for all finishing")
	wg.Wait()

	fmt.Println("Got all chunks")
	chunksData := make([]byte, numChunksHash*fileChunk)

	count := 0
	size := 0
	for chunkPacket := range chunksOutChan {
		for i, hChunk := range chunksHash {
			if hex.EncodeToString(chunkPacket.HashValue) == hex.EncodeToString(hChunk) {
				chunkLen := len(chunkPacket.Data)
				copy(chunksData[i*fileChunk:(i*fileChunk)+chunkLen], chunkPacket.Data)
				gossiper.myFileChunks.Store(hex.EncodeToString(chunkPacket.HashValue), chunkPacket.Data)
				size += chunkLen
				break
			}
		}
		count++
		if count == numChunksHash {
			break
		}
	}

	fileReconstructed := make([]byte, size)
	copy(fileReconstructed, chunksData[:size])

	fmt.Println(len(fileReconstructed))

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

	fmt.Println("Reconstructing file")
	reconstructFileFromChunks(fileName, fileReconstructed)
	fmt.Println("RECONSTRUCTED file " + fileName)

	fileMetadata := &FileMetadata{Name: &fileName, MetaFile: &replyPacket.Data}

	gossiper.myDownloadedFiles.Store(metahashEnc, fileMetadata)
}

func (gossiper *Gossiper) requestMetafilePeriodically(packet *GossipPacket, metaFileChan chan *DataReply) *DataReply {

	metahashEnc := hex.EncodeToString(packet.DataRequest.HashValue)

	go gossiper.forwardDataRequest(packet)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	fmt.Println("Waiting for metafile....")
	for {
		select {
		case replyPacket := <-metaFileChan:
			timer.Stop()
			close(metaFileChan)
			gossiper.hashChannels.Delete(metahashEnc)
			return replyPacket

		case <-timer.C:
			go gossiper.forwardDataRequest(packet)
		}
	}
}

func (gossiper *Gossiper) requestChunkPeriodically(hash []byte, dest string, chanOut chan *DataReply, wg *sync.WaitGroup) {

	defer wg.Done()

	chunkHashEnc := hex.EncodeToString(hash)
	value, _ := gossiper.hashChannels.LoadOrStore(chunkHashEnc, make(chan *DataReply))
	chunkIn := value.(chan *DataReply)

	dataRequest := &DataRequest{Origin: gossiper.name, Destination: dest, HashValue: hash, HopLimit: uint32(hopLimit)}
	newPacket := &GossipPacket{DataRequest: dataRequest}

	fmt.Println(newPacket.DataRequest.Destination)
	go gossiper.forwardDataRequest(newPacket)

	timer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	for {
		select {
		case replyPacket := <-chunkIn:
			fmt.Println("Got chunk")
			timer.Stop()
			fmt.Println("Closing channel")
			close(chunkIn)
			gossiper.hashChannels.Delete(chunkHashEnc)
			fmt.Println("Sending to output channel")
			go func() {
				chanOut <- replyPacket
			}()
			fmt.Println("Done")
			return

		case <-timer.C:
			go gossiper.forwardDataRequest(newPacket)
		}
	}

}

func getChunksFromMetafile(metafile []byte) [][]byte {

	iterations := len(metafile) / 32
	hashes := make([][]byte, iterations)

	for i := 0; i < iterations; i++ {
		hash := metafile[i*32 : (i+1)*32]

		// var hash32 [32]byte
		// copy(hash32[:], hash)
		// //fmt.Println(hash32)
		hashes[i] = hash
		//var hash []byte
		//copy(hash, metafile[i*32:(i+1)*32])
	}

	// fmt.Println(iterations)
	return hashes
}

func reconstructFileFromChunks(name string, chunks []byte) {

	wd, err := os.Getwd()
	helpers.ErrorCheck(err)

	fullPath := wd + downloadFolder + name
	file, err := os.Create(fullPath)
	helpers.ErrorCheck(err)

	defer file.Close()

	_, err = file.Write(chunks)
	helpers.ErrorCheck(err)

	err = file.Sync()
	helpers.ErrorCheck(err)
}

func checkHash(hash []byte, data []byte) bool {

	var hash32 [32]byte
	copy(hash32[:], hash)
	return hash32 == sha256.Sum256(data)
}
