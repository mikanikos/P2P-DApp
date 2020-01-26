package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/mikanikos/Peerster/whisper"
	"os"
	"time"

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
	g := gossiper.NewGossiper(*gossipName, *gossipAddr, helpers.BaseAddress+":"+*uiPort, *peers, *peersNumber)

	w := whisper.NewWhisper(g)

	//for _, peer := range g.PeersData.Peers {
	//	whisper.PeerChannels[peer.String()] = make(chan *gossiper.WhisperPacket, 100)
	//	go w.HandlePeer(peer)
	//}

	// if gui port specified, create and run the webserver (if not, avoid waste of resources for performance reasons)
	if *guiPort != "" {
		ws := webserver.NewWebserver(*uiPort, g)
		go ws.Run(*guiPort)
	}

	// run gossiper
	g.Run()
	fmt.Println("Gossiper running")

	w.Run()
	fmt.Println("Whisper running")
	//w.S

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print(">  ")
		scanner.Scan()
		text := scanner.Text()
		//fmt.Println(text)
		if len(text) != 0 {
			if text == "new key" {
				fmt.Println("Okkkkk")
				key, err := w.GenerateSymKey()
				if err == nil {
					fmt.Println("New key: " + key)
				} else {
					fmt.Println(err)
				}
				//scanner.Scan()
				time.Sleep(time.Duration(10) * time.Second)
				newKeyID := key
				//topic := []byte("maaaaaaaaaaaaa")
				topicType := whisper.ConvertBytesToTopic([]byte("maaaaaaaaaaaaa"))
				fmt.Println(topicType)
				text, _ := hex.DecodeString("ciao andrea")
				newMessage := whisper.NewMessage{
					SymKeyID: newKeyID,
					TTL:      10,
					Topic:    topicType,
					Payload:  text,
					PowTime:  2,
				}
				hash, err := w.NewWhisperMessage(newMessage)
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(hash)
				}
			}
			if text == "add key" {
				scanner.Scan()
				newKey := scanner.Text()
				id, err := w.AddSymKey(newKey)
				if err == nil {
					fmt.Println("Key ID: " + id)
				} else {
					fmt.Println(err)
				}
			}
			if text == "new mess" {
				scanner.Scan()
				newKeyID := scanner.Text()
				topic, _ := hex.DecodeString("ciao")
				text, _ := hex.DecodeString("ciao andrea")
				newMessage := whisper.NewMessage{
					SymKeyID: newKeyID,
					TTL:      30,
					Topic:    whisper.ConvertBytesToTopic(topic),
					Payload:  text,
					PowTime:  2,
				}
				hash, err := w.NewWhisperMessage(newMessage)
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(hash)
				}
			}
			if text == "new sub" {
				scanner.Scan()
				newKeyID := scanner.Text()
				topic1, _ := hex.DecodeString("ciao")
				topic2, _ := hex.DecodeString("miao")
				topics := make([]whisper.Topic, 0)
				topics = append(topics, whisper.ConvertBytesToTopic(topic1))
				topics = append(topics, whisper.ConvertBytesToTopic(topic2))
				crit := whisper.Criteria{
					SymKeyID: newKeyID,
					MinPow: 0.2,
					Topics:   topics,
				}
				hash, err := w.NewMessageFilter(crit)
				if err != nil {
					fmt.Println(err)
				} else {
					fmt.Println(hash)
				}
			}

		} else {
			break
		}
	}

	// wait forever
	select {}
}
