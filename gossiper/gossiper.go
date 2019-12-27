package gossiper

import (
	"fmt"
	"math/rand"
	"net"
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
	name string

	packetChannels map[string]chan *ExtendedGossipPacket

	clientData   *ConnectionData
	gossiperData *ConnectionData
	peersData    *PeersData

	gossipHandler     *GossipHandler
	routingHandler    *RoutingHandler
	fileHandler       *FileHandler
	uiHandler         *UIHandler
	blockchainHandler *BlockchainHandler
}

// NewGossiper constructor
func NewGossiper(name, gossiperAddress, clientAddress, peers string, peersNum uint64) *Gossiper {

	gossiperData, err := createConnectionData(gossiperAddress)
	helpers.ErrorCheck(err)

	clientData, err := createConnectionData(clientAddress)
	helpers.ErrorCheck(err)

	initializeDirectories()

	// create new gossiper
	return &Gossiper{
		name: name,

		// initialize initial channels used throgout the app to exchange messages
		packetChannels: initDefaultChannels(),

		clientData:   clientData,
		gossiperData: gossiperData,
		peersData:    createPeersData(peers, peersNum),

		gossipHandler:     NewGossipHandler(),
		routingHandler:    NewRoutingHandler(),
		fileHandler:       NewFileHandler(),
		uiHandler:         NewUIHandler(),
		blockchainHandler: NewBlockchainHandler(),
	}
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

// Run application
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	// create client channel
	clientChannel := make(chan *helpers.Message, maxChannelSize)
	go gossiper.processClientMessages(clientChannel)

	// start prcessing
	if simpleMode {
		go gossiper.processSimpleMessages()
	} else {
		go gossiper.processStatusMessages()
		go gossiper.processRumorMessages()
		go gossiper.processPrivateMessages()
		go gossiper.processDataRequest()
		go gossiper.processDataReply()
		go gossiper.processSearchRequest()
		go gossiper.processSearchReply()
		go gossiper.processTLCMessage()
		go gossiper.processTLCAck()

		if hw3ex3Mode || hw3ex4Mode {
			go gossiper.processClientBlocks()
		}
	}

	// listen for incoming packets
	go gossiper.receivePacketsFromClient(clientChannel)
	go gossiper.receivePacketsFromPeers()

	if debug {
		fmt.Println("Gossiper running")
	}
}

// create Connection data
func createConnectionData(addressString string) (*ConnectionData, error) {
	// resolve gossiper address
	address, err := net.ResolveUDPAddr("udp4", addressString)
	if err != nil {
		return nil, err
	}

	// get connection for gossiper
	connection, err := net.ListenUDP("udp4", address)
	if err != nil {
		return nil, err
	}

	return &ConnectionData{Address: address, Connection: connection}, nil
}

func initDefaultChannels() map[string]chan *ExtendedGossipPacket {
	// initialize channels used in the application
	channels := make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		if (t != "simple" && !simpleMode) || (t == "simple" && simpleMode) {
			channels[t] = make(chan *ExtendedGossipPacket, maxChannelSize)
		}
	}
	return channels
}

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.name
}

// GetRound of the gossiper
func (gossiper *Gossiper) GetRound() uint32 {
	return gossiper.blockchainHandler.myTime
}

// GetSearchedFiles util
func (gossiper *Gossiper) GetSearchedFiles() []FileGUI {
	return gossiper.uiHandler.filesSearched
}

// GetIndexedFiles util
func (gossiper *Gossiper) GetIndexedFiles() chan *FileGUI {
	return gossiper.uiHandler.filesIndexed
}

// GetDownloadedFiles util
func (gossiper *Gossiper) GetDownloadedFiles() chan *FileGUI {
	return gossiper.uiHandler.filesDownloaded
}

// GetLatestRumorMessages util
func (gossiper *Gossiper) GetLatestRumorMessages() chan *RumorMessage {
	return gossiper.uiHandler.latestRumors
}

// GeBlockchainLogs util
func (gossiper *Gossiper) GeBlockchainLogs() chan string {
	return gossiper.uiHandler.blockchainLogs
}
