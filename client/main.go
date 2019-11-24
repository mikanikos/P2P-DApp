package main

import (
	"flag"

	"github.com/mikanikos/Peerster/client/clientsender"
)

func main() {

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	dest := flag.String("dest", "", "destination for the private message; ​can be omitted")
	msg := flag.String("msg", "", "message to be sent; if the -dest flag is present, this is a private message, otherwise it’s a rumor message")
	file := flag.String("file", "", "file to be indexed by the gossiper")
	request := flag.String("request", "", "request a chunk or metafile of this hash")
	keywords := flag.String("keywords", "", "keywords (comma-separated) to search for files from other peers")
	budget := flag.Uint64("budget", 0, "budget used to search for files in nearby nodes")

	flag.Parse()

	client := clientsender.NewClient(*uiPort)

	client.SendMessage(*msg, dest, file, request, *keywords, *budget)

	client.Conn.Close()
}
