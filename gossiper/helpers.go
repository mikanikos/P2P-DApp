package gossiper

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net"
	"os"

	"github.com/mikanikos/Peerster/helpers"
)

func getTypeFromGossip(packet *GossipPacket) string {
	if packet.Simple != nil {
		return "simple"
	}
	if packet.Rumor != nil {
		return "rumor"
	}
	if packet.Private != nil {
		return "private"
	}

	if packet.Status != nil {
		return "status"
	}

	if packet.DataRequest != nil {
		return "request"
	}

	if packet.DataReply != nil {
		return "reply"
	}

	return "unknown"
}

func (gossiper *Gossiper) getTypeFromMessage(message *helpers.Message) string {
	if gossiper.simpleMode {
		return "simple"
	}

	if message.Destination != nil && message.Text != "" {
		return "private"
	}

	if message.File != nil && message.Destination != nil {
		return "request"
	}

	if message.File != nil {
		return "file"
	}

	return "rumor"
}

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.name
}

func (gossiper *Gossiper) printStatusMessage(extPacket *ExtendedGossipPacket) {
	message := "STATUS from " + extPacket.SenderAddr.String() + " "
	for _, value := range extPacket.Packet.Status.Want {
		message = message + "peer " + value.Identifier + " nextID " + fmt.Sprint(value.NextID) + " "
	}
	fmt.Println(message[:len(message)-1])
	gossiper.printPeers()
}

func (gossiper *Gossiper) printPeerMessage(extPacket *ExtendedGossipPacket) {
	if gossiper.simpleMode {
		fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)
	} else {
		fmt.Println("RUMOR origin " + extPacket.Packet.Rumor.Origin + " from " + extPacket.SenderAddr.String() + " ID " + fmt.Sprint(extPacket.Packet.Rumor.ID) + " contents " + extPacket.Packet.Rumor.Text)
	}
	gossiper.printPeers()
}

func (gossiper *Gossiper) printClientMessage(message *helpers.Message) {
	if message.Destination != nil {
		fmt.Println("CLIENT MESSAGE " + message.Text + " dest " + *message.Destination)
	} else {
		fmt.Println("CLIENT MESSAGE " + message.Text)
	}
	gossiper.printPeers()
}

func (gossiper *Gossiper) getRandomPeer(availablePeers []*net.UDPAddr) *net.UDPAddr {
	indexPeer := rand.Intn(len(availablePeers))
	return availablePeers[indexPeer]

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
