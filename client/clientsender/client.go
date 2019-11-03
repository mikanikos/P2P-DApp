package clientsender

import (
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

// Client struct
type Client struct {
	GossiperAddr *net.UDPAddr
	Conn         *net.UDPConn
}

// NewClient init
func NewClient(uiPort string) *Client {
	gossiperAddr, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)
	conn, err := net.DialUDP("udp4", nil, gossiperAddr)
	helpers.ErrorCheck(err)

	return &Client{
		GossiperAddr: gossiperAddr,
		Conn:         conn,
	}
}

// SendMessage to gossiper
func (client *Client) SendMessage(msg string, dest, file, request *string) {

	packet := helpers.ConvertInputToMessage(msg, dest, file, request)

	//packet := &helpers.Message{Text: msg, Destination: dest, File: file, Request: &decodeRequest}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)

	_, err = client.Conn.Write(packetBytes)
	helpers.ErrorCheck(err)
}
