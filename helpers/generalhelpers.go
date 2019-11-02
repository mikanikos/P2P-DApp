package helpers

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
)

// BaseAddress of the program
const BaseAddress = "127.0.0.1"

// Message struct
type Message struct {
	Text        string
	Destination *string
	File        *string
	Request     *[]byte
}

// ErrorCheck to log errors
func ErrorCheck(err error) {
	if err != nil {
		panic(err)
	}
}

// DifferenceString to do the difference between two string sets
func DifferenceString(list1, list2 []*net.UDPAddr) []*net.UDPAddr {
	mapList2 := make(map[string]struct{}, len(list2))
	for _, x := range list2 {
		mapList2[x.String()] = struct{}{}
	}
	var difference []*net.UDPAddr
	for _, x := range list1 {
		_, check := mapList2[x.String()]
		if !check {
			difference = append(difference, x)
		}
	}
	return difference
}

// GetArrayStringFromAddresses array conversion from address to string
func GetArrayStringFromAddresses(peers []*net.UDPAddr) []string {
	list := make([]string, 0)
	for _, p := range peers {
		list = append(list, p.String())
	}
	return list
}

// ConvertInputToMessage for client arguments
func ConvertInputToMessage(msg string, dest, file, request *string) *Message {
	routeMsg := msg != "" && *dest == "" && *file == "" && *request == ""
	privateMsg := msg != "" && *dest != "" && *file == "" && *request == ""
	fileIndex := msg == "" && *dest == "" && *file != "" && *request == ""
	fileRequest := msg == "" && *dest != "" && *file != "" && *request != ""

	if !(routeMsg || privateMsg || fileIndex || fileRequest) {
		fmt.Println("ERROR (Bad argument combination)")
		os.Exit(1)
	}

	packet := &Message{}

	if routeMsg {
		packet.Text = msg
	}

	if privateMsg {
		packet.Text = msg
		packet.Destination = dest
	}

	if fileIndex {
		packet.File = file
	}

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

	return packet
}
