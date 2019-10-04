package main

import (
	"flag"
)

func main() {

	uiPort := flag.String("UIPort", "8080", "port for the UI client")
	msg := flag.String("msg", "", "message to be sent")

	flag.Parse()

	client := NewClient(*uiPort)

	client.sendMessage(*msg)
}
