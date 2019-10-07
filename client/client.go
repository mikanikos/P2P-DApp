package main

import (
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

//const basePort = "5000"

type Client struct {
	//address         *net.UDPAddr
	gossiperAddr *net.UDPAddr
}

func NewClient(uiPort string) *Client {
	//address, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+basePort)
	//helpers.ErrorCheck(err)
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

	// simpleMsg := &helpers.Message{
	// 	Text: msg,
	// }
	packet := &helpers.Message{Text: msg}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	conn.Write(packetBytes)
}
