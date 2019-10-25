package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

var latestMessagesBuffer = 30
var hopLimit = 10

// Gossiper struct
type Gossiper struct {
	name               string
	channels           map[string]chan *ExtendedGossipPacket
	clientData         *NetworkData
	gossiperData       *NetworkData
	peers              MutexPeers
	simpleMode         bool
	originPackets      PacketsStorage
	myStatus           MutexStatus
	seqID              uint32
	statusChannels     sync.Map
	mongeringChannels  MutexDummyChannel
	antiEntropyTimeout int
	routingTable       MutexRoutingTable
	routeTimer         int
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, simple bool, antiEntropyTimeout int, rtimer int) *Gossiper {
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
		name:               name,
		channels:           initializeChannels(modeTypes, simple),
		clientData:         &NetworkData{Conn: connUI, Addr: addressUI},
		gossiperData:       &NetworkData{Conn: connGossiper, Addr: addressGossiper},
		peers:              MutexPeers{Peers: peers},
		originPackets:      PacketsStorage{OriginPacketsMap: sync.Map{}, LatestMessages: make(chan *RumorMessage, latestMessagesBuffer)},
		myStatus:           MutexStatus{Status: make(map[string]uint32)},
		simpleMode:         simple,
		seqID:              1,
		statusChannels:     sync.Map{},
		mongeringChannels:  MutexDummyChannel{Channels: make(map[string]chan bool)},
		antiEntropyTimeout: antiEntropyTimeout,
		routingTable:       MutexRoutingTable{RoutingTable: make(map[string]*net.UDPAddr)},
		routeTimer:         rtimer,
	}
}

// Run method
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	clientChannel := make(chan *helpers.Message)
	go gossiper.processClientMessages(clientChannel)

	if gossiper.simpleMode {
		go gossiper.processSimpleMessages()
	} else {
		go gossiper.processStatusMessages()
		go gossiper.processRumorMessages()
		go gossiper.processPrivateMessages()
		go gossiper.startAntiEntropy()
		go gossiper.startRouteRumormongering()
	}

	go gossiper.receivePacketsFromClient(clientChannel)
	gossiper.receivePacketsFromPeers()

}

func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range gossiper.channels["private"] {

		if extPacket.Packet.Private.Destination == gossiper.name {
			fmt.Println("PRIVATE origin " + extPacket.Packet.Private.Origin + " hop-limit " + fmt.Sprint(extPacket.Packet.Private.HopLimit) + " contents " + extPacket.Packet.Private.Text)
		} else {
			go gossiper.processPrivateMessage(extPacket)
		}
	}
}

func (gossiper *Gossiper) processClientMessages(clientChannel chan *helpers.Message) {
	for message := range clientChannel {

		gossiper.printClientMessage(message)

		packet := &ExtendedGossipPacket{SenderAddr: gossiper.gossiperData.Addr}

		switch typeMessage := gossiper.getTypeFromMessage(message); typeMessage {
		case "simple":
			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.name, RelayPeerAddr: gossiper.gossiperData.Addr.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.broadcastToPeers(packet)

		case "private":
			privatePacket := &PrivateMessage{Origin: gossiper.name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go gossiper.processPrivateMessage(packet)

		case "rumor":
			id := atomic.LoadUint32(&gossiper.seqID)
			atomic.AddUint32(&gossiper.seqID, uint32(1))
			rumorPacket := &RumorMessage{ID: id, Origin: gossiper.name, Text: message.Text}
			packet.Packet = &GossipPacket{Rumor: rumorPacket}

			gossiper.addMessage(packet)
			go gossiper.startRumorMongering(packet)

		default:
			fmt.Println("Unkown packet!")
		}
	}
}

func (gossiper *Gossiper) processSimpleMessages() {
	for extPacket := range gossiper.channels["simple"] {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printPeerMessage(extPacket)

		extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Addr.String()

		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) processRumorMessages() {
	for extPacket := range gossiper.channels["rumor"] {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printPeerMessage(extPacket)

		gossiper.updateRoutingTable(extPacket)

		isMessageKnown := gossiper.addMessage(extPacket)

		// send status
		statusToSend := gossiper.getStatusToSend()
		gossiper.sendPacket(statusToSend, extPacket.SenderAddr)

		if !isMessageKnown {
			//fmt.Println(gossiper.routingTable.RoutingTable)
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (gossiper *Gossiper) processStatusMessages() {
	for extPacket := range gossiper.channels["status"] {

		gossiper.AddPeer(extPacket.SenderAddr)
		gossiper.printStatusMessage(extPacket)

		channelPeer, exists := gossiper.statusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket))
		if !exists {
			go gossiper.handlePeerStatus(channelPeer.(chan *ExtendedGossipPacket))
		}
		channelPeer.(chan *ExtendedGossipPacket) <- extPacket
	}
}
