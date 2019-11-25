package gossiper

import (
	"crypto/sha256"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"sort"

	"github.com/mikanikos/Peerster/helpers"
)

func initializeChannels(modeTypes []string) (channels map[string]chan *ExtendedGossipPacket) {
	channels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		if (t != "simple" && !simpleMode) || (t == "simple" && simpleMode) {
			channels[t] = make(chan *ExtendedGossipPacket)
		}
	}
	return channels
}

func getTypeFromGossip(packet *GossipPacket) string {

	// as stated in the handout, we assume every packet has only one field which is not null

	if packet.Simple != nil {
		return "simple"
	} else if packet.Rumor != nil {
		return "rumor"
	} else if packet.Private != nil {
		return "private"
	} else if packet.Status != nil {
		return "status"
	} else if packet.DataRequest != nil {
		return "dataRequest"
	} else if packet.DataReply != nil {
		return "dataReply"
	} else if packet.SearchRequest != nil {
		return "searchRequest"
	} else if packet.SearchReply != nil {
		return "searchReply"
	} else if packet.TLCMessage != nil {
		return "tlcMes"
	} else if packet.Ack != nil {
		return "tlcAck"
	}
	return "unknown"
}

func getTypeFromMessage(message *helpers.Message) string {
	if simpleMode {
		return "simple"
	}

	if message.Destination != nil && message.Text != "" && message.File == nil && message.Request == nil && message.Keywords == nil {
		return "private"
	}

	if message.Text == "" && message.File != nil && message.Request != nil && message.Keywords == nil {
		return "dataRequest"
	}

	if message.Destination == nil && message.Text == "" && message.File != nil && message.Request == nil && message.Keywords == nil {
		return "file"
	}

	if message.Destination == nil && message.Text == "" && message.File == nil && message.Request == nil && message.Keywords != nil {
		return "searchRequest"
	}

	return "rumor"
}

func getRandomPeer(availablePeers []*net.UDPAddr) *net.UDPAddr {
	indexPeer := rand.Intn(len(availablePeers))
	return availablePeers[indexPeer]
}

func getChunksFromMetafile(metafile []byte) [][]byte {
	iterations := len(metafile) / 32
	hashes := make([][]byte, iterations)
	for i := 0; i < iterations; i++ {
		hash := metafile[i*32 : (i+1)*32]
		hashes[i] = hash
	}
	return hashes
}

func checkHash(hash []byte, data []byte) bool {

	var hash32 [32]byte
	copy(hash32[:], hash)
	value := sha256.Sum256(data)
	return hash32 == value
}

func copyFile(source, target string) {

	source = downloadFolder + source
	target = downloadFolder + target

	sourceFile, err := os.Open(source)
	helpers.ErrorCheck(err)
	defer sourceFile.Close()

	targetFile, err := os.Create(target)
	helpers.ErrorCheck(err)
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	helpers.ErrorCheck(err)

	err = targetFile.Close()
	helpers.ErrorCheck(err)
}

func sortUint64(slice []uint64) {
	sort.Slice(slice, func(i, j int) bool { return slice[i] < slice[j] })
}

func insertToSortUint64Slice(data []uint64, el uint64) []uint64 {
	index := sort.Search(len(data), func(i int) bool { return data[i] > el })
	data = append(data, 0)
	copy(data[index+1:], data[index:])
	data[index] = el
	return data
}

func saveFileOnDisk(fileName string, data []byte) {
	file, err := os.Create(downloadFolder + fileName)
	helpers.ErrorCheck(err)
	defer file.Close()

	_, err = file.Write(data)
	helpers.ErrorCheck(err)

	err = file.Sync()
	helpers.ErrorCheck(err)
}

func chunksIntegrityCheck(fileMetadata *FileMetadata) bool {

	if fileMetadata.ChunkCount != uint64(len(fileMetadata.ChunkMap)) {
		return false
	}

	for i := uint64(0); i < fileMetadata.ChunkCount; i++ {
		if fileMetadata.ChunkMap[i] != i+1 {
			if debug {
				fmt.Println("ERROR: not all chunks retrieved")
			}
			return false
		}
	}
	return true
}
