package main

import (
	"flag"

	"github.com/mikanikos/Peerster/gossiper"
	"github.com/mikanikos/Peerster/helpers"
	"github.com/mikanikos/Peerster/webserver"
)

// main entry point of the peerster app
func main() {

	// parsing arguments according to the specification given
	guiPort := flag.String("GUIPort", "", "port for the graphical interface")
	uiPort := flag.String("UIPort", "8080", "port for the command line interface")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	gossipName := flag.String("name", "", "name of the gossiper")
	peers := flag.String("peers", "", "comma separated list of peers of the form ip:port")
	peersNumber := flag.Uint64("N", 1, "total number of peers in the network")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	hw3ex2 := flag.Bool("hw3ex2", false, "enable gossiper mode for knowing the transactions from other peers")
	hw3ex3 := flag.Bool("hw3ex3", false, "enable gossiper mode for round based gossiping")
	hw3ex4 := flag.Bool("hw3ex4", false, "enable gossiper mode for consensus agreement")
	ackAll := flag.Bool("ackAll", false, "make gossiper ack all tlc messages regardless of the ID")
	antiEntropy := flag.Uint("antiEntropy", 10, "timeout in seconds for anti-entropy")
	rtimer := flag.Uint("rtimer", 0, "timeout in seconds to send route rumors")
	hopLimit := flag.Uint("hopLimit", 10, "hop limit value (TTL) for a packet")
	stubbornTimeout := flag.Uint("stubbornTimeout", 5, "stubborn timeout to resend a txn BlockPublish until it receives a majority of acks")

	flag.Parse()

	// set flags that are used througout the application
	gossiper.SetAppConstants(*simple, *hw3ex2, *hw3ex3, *hw3ex4, *ackAll, *hopLimit, *stubbornTimeout, *rtimer, *antiEntropy)

	// create new gossiper instance
	gossiper := gossiper.NewGossiper(*gossipName, *gossipAddr, helpers.BaseAddress+":"+*uiPort, *peers, *peersNumber)

	// if gui port specified, create and run the webserver (if not, avoid waste of resources for performance reasons)
	if *guiPort != "" {
		webserver := webserver.NewWebserver(*uiPort, gossiper)
		go webserver.Run(*guiPort)
	}

	// run gossiper
	gossiper.Run()

	// wait forever
	select {}
}
