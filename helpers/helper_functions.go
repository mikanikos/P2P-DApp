package helpers

import (
	"fmt"
	"net"
)

const BaseAddress = "127.0.0.1"

// SimpleMessage struct
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

// GossipPacket struct
type GossipPacket struct {
	Simple *SimpleMessage
}

type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

func ErrorCheck(err error) {
	if err != nil {
		fmt.Println(err)
	}
}
