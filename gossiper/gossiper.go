package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// Gossiper struct
type Gossiper struct {
	name               string
	clientData         *NetworkData
	gossiperData       *NetworkData
	peers              MutexPeers
	simpleMode         bool
	originPackets      PacketsStorage
	seqID              MutexSequenceID
	statusChannels     map[string]chan *ExtendedGossipPacket
	antiEntropyTimeout int
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

	originPackets := make(map[string]map[uint32]*ExtendedGossipPacket)

	return &Gossiper{
		name:               name,
		clientData:         &NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData:       &NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peers:              MutexPeers{Peers: peers},
		originPackets:      PacketsStorage{OriginPacketsMap: originPackets, Messages: make([]RumorMessage, 0)},
		simpleMode:         simple,
		seqID:              MutexSequenceID{ID: 1},
		statusChannels:     make(map[string]chan *ExtendedGossipPacket),
		antiEntropyTimeout: antiEntropyTimeout,
	}
}

// Run function
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

		gossiper.addMessage(extPacket)

		// broadcast
		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) handleConnectionRumor(rumorChannel chan *ExtendedGossipPacket) {
	for extPacket := range rumorChannel {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printPeerMessage(extPacket)

		isMessageKnown := gossiper.addMessage(extPacket)

		go gossiper.sendStatusPacket(extPacket.SenderAddr)

		if !isMessageKnown {
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (gossiper *Gossiper) handleConnectionStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		gossiper.notifyStatusChannel(extPacket)

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printStatusMessage(extPacket)

		myStatus := gossiper.createStatus()

		toSend := gossiper.getDifferenceStatus(myStatus, extPacket.Packet.Status.Want)
		wanted := gossiper.getDifferenceStatus(extPacket.Packet.Status.Want, myStatus)

		gossiper.sendPacketsFromStatus(toSend, extPacket.SenderAddr)

		if len(wanted) != 0 {
			go gossiper.sendStatusPacket(extPacket.SenderAddr)
		} else {
			if len(toSend) == 0 {
				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
			}
		}
	}
}
