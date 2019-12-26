package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// Gossiper struct
type Gossiper struct {
	name string

	packetChannels map[string]chan *ExtendedGossipPacket

	clientData   *NetworkData
	gossiperData *NetworkData
	peersData    *PeersData

	gossipHandler     *GossipHandler
	routingHandler    *RoutingHandler
	fileHandler       *FileHandler
	uiHandler         *UIHandler
	blockchainHandler *BlockchainHandler
}

// NewGossiper constructor
func NewGossiper(name string, address string, peersList []string, uiPort string, peersNum uint64) *Gossiper {

	// resolve gossiper address
	addressGossiper, err := net.ResolveUDPAddr("udp4", address)
	helpers.ErrorCheck(err)

	// get connection for gossiper
	connGossiper, err := net.ListenUDP("udp4", addressGossiper)
	helpers.ErrorCheck(err)

	// resolve client address
	addressUI, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err)

	// get connection for client
	connUI, err := net.ListenUDP("udp4", addressUI)
	helpers.ErrorCheck(err)

	// resolve peers addresses given
	peers := make([]*net.UDPAddr, 0)
	for _, peer := range peersList {
		addressPeer, err := net.ResolveUDPAddr("udp4", peer)
		if err == nil {
			peers = append(peers, addressPeer)
		}
	}

	// create new gossiper
	return &Gossiper{
		name: name,

		// initialize channels used throgout the app to exchange messages
		packetChannels: initializeChannels(modeTypes),

		clientData:   &NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData: &NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peersData:    &PeersData{Peers: peers, Size: peersNum},

		gossipHandler:     NewGossipHandler(),
		routingHandler:    NewRoutingHandler(),
		fileHandler:       NewFileHandler(),
		uiHandler:         NewUIHandler(),
		blockchainHandler: NewBlockchainHandler(),
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

	// if qsc, set tlc too
	if hw3ex4Mode {
		hw3ex3Mode = true
	}

	// if tlc, set gossip with confirmation too
	if hw3ex3Mode {
		hw3ex3Mode = true
	}

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
		if hw3ex3Mode || hw3ex4Mode {
			go gossiper.processClientBlocks()
		}
	}

	// listen for incoming packets 
	go gossiper.receivePacketsFromClient(clientChannel)

	if debug {
		fmt.Println("Gossiper running")
	}

	gossiper.receivePacketsFromPeers()
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

// PeersData struct
type PeersData struct {
	Peers []*net.UDPAddr
	Size  uint64
	Mutex sync.RWMutex
}

// AddPeer to peers list if not present
func (gossiper *Gossiper) AddPeer(peer *net.UDPAddr) {
	gossiper.peersData.Mutex.Lock()
	defer gossiper.peersData.Mutex.Unlock()
	contains := false
	for _, p := range gossiper.peersData.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		gossiper.peersData.Peers = append(gossiper.peersData.Peers, peer)
	}
}

// GetPeersAtomic in concurrent environment
func (gossiper *Gossiper) GetPeersAtomic() []*net.UDPAddr {
	gossiper.peersData.Mutex.RLock()
	defer gossiper.peersData.Mutex.RUnlock()
	peerCopy := make([]*net.UDPAddr, len(gossiper.peersData.Peers))
	copy(peerCopy, gossiper.peersData.Peers)
	return peerCopy
}
