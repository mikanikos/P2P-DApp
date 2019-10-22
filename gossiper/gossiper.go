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
	name              string
	clientData        *NetworkData
	gossiperData      *NetworkData
	peers             MutexPeers
	simpleMode        bool
	originPackets     PacketsStorage
	seqID             uint32
	statusChannels    sync.Map //MutexStatusChannel
	mongeringChannels MutexDummyChannel
	//syncChannels       MutexDummyChannel
	antiEntropyTimeout int
	//isMongering        map[string]bool
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, simple bool, antiEntropyTimeout int) *Gossiper {
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
		clientData:        &NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData:      &NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peers:             MutexPeers{Peers: peers},
		originPackets:     PacketsStorage{OriginPacketsMap: sync.Map{}, LatestMessages: make(chan *RumorMessage, 30)}, //PacketsStorage{OriginPacketsMap: make(map[string]map[uint32]*ExtendedGossipPacket), Messages: make([]RumorMessage, 0)},
		simpleMode:        simple,
		seqID:             1,
		statusChannels:    sync.Map{}, //MutexStatusChannel{Channels: make(map[string]chan *ExtendedGossipPacket)},
		mongeringChannels: MutexDummyChannel{Channels: make(map[string]chan bool)},
		//syncChannels:       MutexDummyChannel{Channels: make(map[string]chan bool)},
		antiEntropyTimeout: antiEntropyTimeout,
		//isMongering:        make(map[string]bool),
	}
}

// Run method
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	channels := initializeChannels(modeTypes)

	go gossiper.handleConnectionClient(channels["client"])
	go gossiper.receivePackets(gossiper.clientData, channels)

	if gossiper.simpleMode {
		go gossiper.handleConnectionSimple(channels["simple"])
	} else {
		go gossiper.handleConnectionStatus(channels["status"])
		go gossiper.handleConnectionRumor(channels["rumor"])
		go gossiper.startAntiEntropy()
	}

	gossiper.receivePackets(gossiper.gossiperData, channels)

}

func (gossiper *Gossiper) handleConnectionClient(channelClient chan *ExtendedGossipPacket) {
	for extPacket := range channelClient {

		gossiper.printClientMessage(extPacket)

		extPacket = gossiper.modifyPacket(extPacket, true)

		gossiper.addMessage(extPacket)

		if gossiper.simpleMode {
			go gossiper.broadcastToPeers(extPacket)
		} else {
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (gossiper *Gossiper) handleConnectionSimple(channelPeers chan *ExtendedGossipPacket) {
	for extPacket := range channelPeers {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printPeerMessage(extPacket)

		extPacket = gossiper.modifyPacket(extPacket, false)

		//gossiper.addMessage(extPacket)

		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) handleConnectionRumor(rumorChannel chan *ExtendedGossipPacket) {
	for extPacket := range rumorChannel {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printPeerMessage(extPacket)

		isMessageKnown := gossiper.addMessage(extPacket)

		gossiper.sendStatusPacket(extPacket.SenderAddr)

		if !isMessageKnown {
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (gossiper *Gossiper) handleConnectionStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		//time.Sleep(time.Duration(3) * time.Second)

		//gossiper.createOrGetMongerCond(extPacket.SenderAddr.String()).Broadcast()

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printStatusMessage(extPacket)

		// if gossiper.isMongering[extPacket.SenderAddr.String()] {

		//time.Sleep(time.Duration(15) * time.Second)

		go gossiper.sendToPeerStatusChannel(extPacket)
	}
}
