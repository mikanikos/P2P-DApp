package main

import (
	"flag"
	"strings"

	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/webserver"
)

func main() {

	guiPort := flag.String("GUIPort", "", "port for the graphical interface")
	uiPort := flag.String("UIPort", "8080", "port for the command line interface")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	gossipName := flag.String("name", "", "name of the gossiper")
	peers := flag.String("peers", "", "comma separated list of peers of the form ip:port")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	antiEntropy := flag.Int("antiEntropy", 10, "timeout in seconds for anti-entropy")
	rtimer := flag.Int("rtimer", 0, "timeout in seconds to send route rumors")

	flag.Parse()

	peersList := make([]string, 0)
	if *peers != "" {
		peersList = strings.Split(*peers, ",")
	}

	gossiper := gossiper.NewGossiper(*gossipName, *gossipAddr, peersList, *uiPort, *simple)

	if *guiPort != "" {
		webserver := webserver.NewWebserver(*uiPort, gossiper)
		go webserver.Run(*guiPort)
	}

	if !*simple {
		go gossiper.StartAntiEntropy(*antiEntropy)
		go gossiper.StartRouteRumormongering(*rtimer)
	}

	gossiper.Run()
}
