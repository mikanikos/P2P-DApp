package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// ConnectionData struct: connection + address
type ConnectionData struct {
	Connection *net.UDPConn
	Address    *net.UDPAddr
}

// Gossiper struct
type Gossiper struct {
	// name of the gossiper
	Name string
	// save peers data (neighbours and total number of peers in the network)
	PeersData *PeersData

	// handle connection with peers and client
	ConnectionHandler *ConnectionHandler
	// handle gossip messages and status messages
	gossipHandler *GossipHandler
	// handle routing table and forwarding
	routingHandler *RoutingHandler
	// handle file indexing, sharing and searching
	fileHandler *FileHandler
	// handle abstractions for the blockchain (gossip with confirmation, tlc and qsc)
	blockchainHandler *BlockchainHandler
}

// NewGossiper constructor
func NewGossiper(name, gossiperAddress, clientAddress, peers string, peersNum uint64) *Gossiper {

	// init app
	Init()

	gossiper := &Gossiper{
		Name:      name,
		PeersData: createPeersData(peers, peersNum),

		ConnectionHandler: NewConnectionHandler(gossiperAddress, clientAddress),
		gossipHandler:     NewGossipHandler(),
		routingHandler:    NewRoutingHandler(),
		fileHandler:       NewFileHandler(),
		blockchainHandler: NewBlockchainHandler(),
	}

	return gossiper

}

// SetAppConstants based on parameters
func SetAppConstants(simple, hw3ex2, hw3ex3, hw3ex4, ackAll bool, hopLimitVal, stubbornTimeoutVal, rtimer, antiEntropy uint) {
	simpleMode = simple
	hw3ex2Mode = hw3ex2
	hw3ex3Mode = hw3ex3
	hw3ex4Mode = hw3ex4
	ackAllMode = ackAll

	// if qsc, set tlc too
	if hw3ex4Mode {
		hw3ex3Mode = true
	}

	// if tlc, set gossip with confirmation too
	if hw3ex3Mode {
		hw3ex3Mode = true
	}

	hopLimit = int(hopLimitVal)
	stubbornTimeout = int(stubbornTimeoutVal)
	routeRumorTimeout = int(rtimer)
	antiEntropyTimeout = int(antiEntropy)
}

// Init app structures and environments
func Init() {
	// initialize channels used to exchange packets in the app
	initPacketChannels()

	// create working directories for shared and downloaded files
	createWorkingDirectories()
}

// initPacketChannels that are used in the app
func initPacketChannels() {
	// initialize channels used in the application
	PacketChannels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		if (t != "simple" && !simpleMode) || (t == "simple" && simpleMode) {
			PacketChannels[t] = make(chan *ExtendedGossipPacket, maxChannelSize)
		}
	}
}

// createWorkingDirectories for shared and downloaded files at the base directory
func createWorkingDirectories() {
	wd, err := os.Getwd()
	helpers.ErrorCheck(err, true)

	shareFolder = wd + shareFolder
	downloadFolder = wd + downloadFolder

	os.Mkdir(shareFolder, os.ModePerm)
	os.Mkdir(downloadFolder, os.ModePerm)
}

// Run application
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	// create client channel
	clientChannel := make(chan *helpers.Message, maxChannelSize)
	go gossiper.processClientMessages(clientChannel)

	// start processing on separate goroutines

	go gossiper.processSimpleMessages()
	go gossiper.processStatusMessages()
	go gossiper.processRumorMessages()
	go gossiper.startAntiEntropy()

	go gossiper.startRouteRumormongering()
	go gossiper.processPrivateMessages()

	go gossiper.processDataRequest()
	go gossiper.processDataReply()

	go gossiper.processSearchRequest()
	go gossiper.processSearchReply()

	go gossiper.processTLCMessage()
	go gossiper.processTLCAck()
	go gossiper.handleTLCMessage()

	go gossiper.processClientBlocks()

	// listen for incoming packets
	go gossiper.receivePacketsFromClient(clientChannel)
	go gossiper.receivePacketsFromPeers()

	if debug {
		fmt.Println("Gossiper running")
	}

	//api := whisper.NewPublicWhisperAPI(gossiper.whisper)
	//
	//scanner := bufio.NewScanner(os.Stdin)
	//for {
	//    fmt.Print("Enter Text: ")
	//    // Scans a line from Stdin(Console)
	//    scanner.Scan()
	//    // Holds the string that scanned
	//	text := scanner.Text()
	//	fmt.Println(text)
	//    if len(text) != 0 {
	//        if text == "new key" {
	//			fmt.Println("Okkkkk")
	//			key, err := api.NewSymKey()
	//			if err == nil {
	//				fmt.Println("New key: " + key)
	//			} else {
	//				fmt.Println(err)
	//			}
	//			//scanner.Scan()
	//			time.Sleep(time.Duration(3) * time.Second)
	//			newKeyID := key
	//			//topic := []byte("maaaaaaaaaaaaa")
	//			topicType := whisper.BytesToTopic([]byte("maaaaaaaaaaaaa"))
	//			fmt.Println(topicType)
	//			text, _ := hex.DecodeString("ciao andrea")
	//			newMessage := whisper.NewMessage{
	//				SymKeyID:   newKeyID,
	//				TTL:        10,
	//				Topic:      topicType,
	//				Payload:    text,
	//				PowTime:    2,
	//				Pow:  2.01,
	//			}
	//			hash, err := api.NewWhisperMessage(newMessage)
	//			if err != nil {
	//				fmt.Println(err)
	//			} else {
	//				fmt.Println(hash)
	//			}
	//		}
	//		if text == "add key" {
	//			scanner.Scan()
	//			newKey := scanner.Text()
	//			id, err := api.AddSymKey(newKey)
	//			if err == nil {
	//				fmt.Println("Key ID: " + id)
	//			} else {
	//				fmt.Println(err)
	//			}
	//		}
	//		if text == "new mess" {
	//			scanner.Scan()
	//			newKeyID := scanner.Text()
	//			topic, _ := hex.DecodeString("ciao")
	//			text, _ := hex.DecodeString("ciao andrea")
	//			newMessage := whisper.NewMessage{
	//				SymKeyID:  newKeyID,
	//				TTL:       30,
	//				Topic:     whisper.BytesToTopic(topic),
	//				Payload:   text,
	//				PowTime:   2,
	//				Pow: 2.01,
	//			}
	//			hash, err := api.NewWhisperMessage(newMessage)
	//			if err != nil {
	//				fmt.Println(err)
	//			} else {
	//				fmt.Println(hash)
	//			}
	//		}
	//    } else {
	//        break
	//    }
	//}

}

// getters

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.Name
}

// GetRound of the gossiper
func (gossiper *Gossiper) GetRound() uint32 {
	return gossiper.blockchainHandler.myTime
}

// GetSearchedFiles util
func (gossiper *Gossiper) GetSearchedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesSearched
}

// GetIndexedFiles util
func (gossiper *Gossiper) GetIndexedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesIndexed
}

// GetDownloadedFiles util
func (gossiper *Gossiper) GetDownloadedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesDownloaded
}

// GetLatestRumorMessages util
func (gossiper *Gossiper) GetLatestRumorMessages() chan *RumorMessage {
	return gossiper.gossipHandler.latestRumors
}

// GetBlockchainLogs util
func (gossiper *Gossiper) GetBlockchainLogs() chan string {
	return gossiper.blockchainHandler.blockchainLogs
}
