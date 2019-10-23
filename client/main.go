package main

import (
	"flag"
)

func main() {

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	dest := flag.String("dest", "", "destination for the private message; ​can be omitted")
	msg := flag.String("msg", "", "message to be sent; if the -dest flag is present, this is a private message, otherwise it’s a rumor message")

	flag.Parse()

	client := NewClient(*uiPort)

	client.sendMessage(*msg, dest)
}
