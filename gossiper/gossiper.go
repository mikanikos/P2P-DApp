package gossiper

import (
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// Gossiper struct
type Gossiper struct {
	name string

	channels map[string]chan *ExtendedGossipPacket

	clientData   *NetworkData
	gossiperData *NetworkData

	peers       MutexPeers
	peersNumber uint64
	origins     MutexOrigins

	messageStorage sync.Map
	myStatus       MutexStatus

	seqID  uint32
	myTime uint32

	statusChannels    sync.Map
	mongeringChannels sync.Map

	routingTable MutexRoutingTable
	originLastID MutexStatus

	fileHandler *FileHandler
	uiHandler   *UIHandler

	tlcAckChan     chan *TLCAck
	tlcConfirmChan chan *TLCMessage

	clientBlockBuffer chan BlockPublish
	tlcStatus         MutexStatus

	// firstTLCDone  bool
	// confirmations map[string]uint32
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, peersNum uint64) *Gossiper {

	addressGossiper, err := net.ResolveUDPAddr("udp4", address)
	helpers.ErrorCheck(err)
	connGossiper, err := net.ListenUDP("udp4", addressGossiper)
	helpers.ErrorCheck(err)
	addressUI, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)
	connUI, err := net.ListenUDP("udp4", addressUI)
	helpers.ErrorCheck(err)

	peers := make([]*net.UDPAddr, 0)
	for _, peer := range peersList {
		addressPeer, err := net.ResolveUDPAddr("udp4", peer)
		if err == nil {
			peers = append(peers, addressPeer)
		}
	}

	return &Gossiper{
		name:              name,
		channels:          initializeChannels(modeTypes),
		clientData:        &NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData:      &NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peers:             MutexPeers{Peers: peers},
		peersNumber:       peersNum,
		origins:           MutexOrigins{Origins: make([]string, 0)},
		messageStorage:    sync.Map{},
		myStatus:          MutexStatus{Status: make(map[string]uint32)},
		originLastID:      MutexStatus{Status: make(map[string]uint32)},
		seqID:             1,
		myTime:            0,
		statusChannels:    sync.Map{},
		mongeringChannels: sync.Map{},
		routingTable:      MutexRoutingTable{RoutingTable: make(map[string]*net.UDPAddr)},
		fileHandler:       NewFileHandler(),
		uiHandler:         NewUIHandler(),
		tlcAckChan:        make(chan *TLCAck, maxChannelSize),
		tlcConfirmChan:    make(chan *TLCMessage, maxChannelSize),
		clientBlockBuffer: make(chan BlockPublish, maxChannelSize),
		tlcStatus:         MutexStatus{Status: make(map[string]uint32)},
		// firstTLCDone:      false,
		// confirmations:     make(map[string]uint32),
	}
}

// SetConstantValues based on parameters
func SetConstantValues(simple, hw3ex2, hw3ex3, hw3ex4 bool, hopLimitVal, stubbornTimeoutVal uint, ackAll bool) {
	simpleMode = simple
	hw3ex2Mode = hw3ex2
	hw3ex3Mode = hw3ex3
	hw3ex4Mode = hw3ex4
	hopLimit = int(hopLimitVal)
	stubbornTimeout = int(stubbornTimeoutVal)
	ackAllMode = ackAll
}

// Run method
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	clientChannel := make(chan *helpers.Message, maxChannelSize)
	go gossiper.processClientMessages(clientChannel)

	if simpleMode {
		go gossiper.processSimpleMessages()
	} else {
		initializeDirectories()

		go gossiper.processStatusMessages()
		go gossiper.processRumorMessages()
		go gossiper.processPrivateMessages()
		go gossiper.processDataRequest()
		go gossiper.processDataReply()
		go gossiper.processSearchRequest()
		go gossiper.processSearchReply()
		go gossiper.processTLCMessage()
		go gossiper.processTLCAck()
		if hw3ex2Mode || hw3ex3Mode {
			go gossiper.processClientBlocks()
		}
	}

	go gossiper.receivePacketsFromClient(clientChannel)
	gossiper.receivePacketsFromPeers()
}

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.name
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
