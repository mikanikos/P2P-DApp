package helpers

import (
	"fmt"
	"net"
)

const BaseAddress = "127.0.0.1"

// Message struct
type Message struct {
	Text string
}

// SimpleMessage struct
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

// GossipPacket struct
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

type NetworkData struct {
	Conn *net.UDPConn
	Addr *net.UDPAddr
}

type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

type StatusPacket struct {
	Want []PeerStatus
}

type PeerStatus struct {
	Identifier string
	NextID     uint32
}

func ErrorCheck(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

func GetTypeMode(packet *GossipPacket) string {
	if packet.Simple != nil {
		return "simple"
	}
	if packet.Rumor != nil {
		return "rumor"
	}
	return "status"
}
