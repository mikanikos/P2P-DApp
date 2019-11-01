package clientsender

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
		gossiperAddr: gossiperAddr,
	}
}

// SendMessage to gossiper
func (client *Client) SendMessage(msg string, dest, file, request *string) {
	conn, err := net.DialUDP("udp", nil, client.gossiperAddr)
	helpers.ErrorCheck(err)
	defer conn.Close()

	packet := helpers.ConvertInputToMessage(msg, dest, file, request)

	//packet := &helpers.Message{Text: msg, Destination: dest, File: file, Request: &decodeRequest}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	conn.Write(packetBytes)
}
