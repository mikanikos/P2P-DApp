package gossiper

import (
	"fmt"
	"math/rand"
	"net"

	"github.com/mikanikos/Peerster/helpers"
)

var modeTypes = []string{"simple", "rumor", "status", "private", "request", "reply"}

// SimpleMessage struct
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

// GossipPacket struct
type GossipPacket struct {
	Simple      *SimpleMessage
	Rumor       *RumorMessage
	Status      *StatusPacket
	Private     *PrivateMessage
	DataRequest *DataRequest
	DataReply   *DataReply
}

// NetworkData struct
type NetworkData struct {
	Conn *net.UDPConn
	Addr *net.UDPAddr
}

// ExtendedGossipPacket struct
type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

// PrivateMessage struct
type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

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

	if *message.Destination != "" && message.Text != "" {
		return "private"
	}

	if *message.File != "" && *message.Destination != "" {
		return "request"
	}

	if *message.File != "" {
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
	if *message.Destination != "" {
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
