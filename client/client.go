package main

import (
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

// Client struct
type Client struct {
	gossiperAddr *net.UDPAddr
}

// NewClient init
func NewClient(uiPort string) *Client {
	gossiperAddr, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)

	return &Client{
		//	address:            address,
		gossiperAddr: gossiperAddr,
	}
}

func (client *Client) sendMessage(msg string) {
	conn, err := net.DialUDP("udp", nil, client.gossiperAddr)
	helpers.ErrorCheck(err)
	defer conn.Close()

	packet := &helpers.Message{Text: msg}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	conn.Write(packetBytes)
}
