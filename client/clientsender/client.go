package clientsender

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"

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

	routeMsg := msg != "" && *dest == "" && *file == "" && *request == ""
	privateMsg := msg != "" && *dest != "" && *file == "" && *request == ""
	fileIndex := msg == "" && *dest == "" && *file != "" && *request == ""
	fileRequest := msg == "" && *dest != "" && *file != "" && *request != ""

	if !(routeMsg || privateMsg || fileIndex || fileRequest) {
		fmt.Println("ERROR (Bad argument combination)")
		os.Exit(1)
	}

	packet := &helpers.Message{Text: msg, Destination: dest, File: file}

	if fileRequest {
		decodeRequest, err := hex.DecodeString(*request)
		if err != nil {
			fmt.Println("ERROR (Unable to decode hex hash)")
			os.Exit(1)
		}
		packet.Request = &decodeRequest
		packet.Destination = dest
		packet.File = file
	}

	//packet := &helpers.Message{Text: msg, Destination: dest, File: file, Request: &decodeRequest}
	packetBytes, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	conn.Write(packetBytes)
}
