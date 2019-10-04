package gossiper

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/dedis/protobuf"

	"github.com/mikanikos/Peerster/helpers"
)

var maxBufferSize = 1024

type SafePeers struct {
	Peers []*net.UDPAddr
	Mutex sync.Mutex
}

// Gossiper struct
type Gossiper struct {
	name            string
	addressUI       *net.UDPAddr
	connUI          *net.UDPConn
	addressGossiper *net.UDPAddr
	connGossiper    *net.UDPConn
	peers           SafePeers
	simpleMode      bool
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, simple bool) *Gossiper {
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
		helpers.ErrorCheck(err)
		peers = append(peers, addressPeer)
	}

	return &Gossiper{
		name:            name,
		addressGossiper: addressGossiper,
		connGossiper:    connGossiper,
		addressUI:       addressUI,
		connUI:          connUI,
		peers:           SafePeers{Peers: peers},
		simpleMode:      simple,
	}
}

// Run function
func (gossiper *Gossiper) Run() {
	defer gossiper.connGossiper.Close()
	defer gossiper.connUI.Close()

	clientChannel := make(chan *helpers.ExtendedGossipPacket)
	peersChannel := make(chan *helpers.ExtendedGossipPacket)

	defer close(clientChannel)
	defer close(peersChannel)

	go gossiper.handleConnectionClient(clientChannel)
	go getPacket(gossiper.connUI, clientChannel)

	go gossiper.handleConnectionPeers(peersChannel)
	getPacket(gossiper.connGossiper, peersChannel)

}

func (gossiper *Gossiper) handleConnectionClient(channelClient chan *helpers.ExtendedGossipPacket) {

	for extPacket := range channelClient {

		// print messages
		fmt.Println("CLIENT MESSAGE " + extPacket.Packet.Simple.Contents)
		printPeers(gossiper.peers.getPeersCopy())

		// modify packet
		extPacket.Packet.Simple.OriginalName = gossiper.name
		extPacket.Packet.Simple.RelayPeerAddr = gossiper.addressGossiper.String()

		// broadcast
		gossiper.broadcastToPeers(extPacket)
	}
}

func (peers *SafePeers) addPeer(peer *net.UDPAddr) {
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	contains := false
	for _, p := range peers.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		peers.Peers = append(peers.Peers, peer)
	}
}

func printPeers(peers []*net.UDPAddr) {
	list := make([]string, 0)
	for _, p := range peers {
		list = append(list, p.String())
	}
	fmt.Println("PEERS " + strings.Join(list, ","))
}

func (peers *SafePeers) getPeersCopy() []*net.UDPAddr {
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	peerCopy := make([]*net.UDPAddr, len(peers.Peers))
	copy(peerCopy, peers.Peers)
	return peerCopy
}

func (gossiper *Gossiper) handleConnectionPeers(channelPeers chan *helpers.ExtendedGossipPacket) {
	for extPacket := range channelPeers {

		// print messages
		fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)

		// get receiver address and store it
		gossiper.peers.addPeer(extPacket.SenderAddr)
		printPeers(gossiper.peers.getPeersCopy())

		// change address in packet
		extPacket.Packet.Simple.RelayPeerAddr = gossiper.addressGossiper.String()

		// broadcast
		gossiper.broadcastToPeers(extPacket)
	}
}

func getPacket(conn *net.UDPConn, channel chan *helpers.ExtendedGossipPacket) {
	for {
		packet := &helpers.GossipPacket{}
		packetBytes := make([]byte, maxBufferSize)
		_, addr, err := conn.ReadFromUDP(packetBytes)
		// if n > maxBufferSize {
		// 	maxBufferSize = maxBufferSize * 2
		// }
		helpers.ErrorCheck(err)
		protobuf.Decode(packetBytes, packet)
		channel <- &helpers.ExtendedGossipPacket{Packet: packet, SenderAddr: addr}
	}
}

func (gossiper *Gossiper) broadcastToPeers(packet *helpers.ExtendedGossipPacket) {
	packetToSend, err := protobuf.Encode(packet.Packet)
	helpers.ErrorCheck(err)

	// broadcast to peers
	for _, peer := range gossiper.peers.getPeersCopy() {
		if peer.String() != packet.SenderAddr.String() {
			gossiper.connGossiper.WriteToUDP(packetToSend, peer)
		}
	}
}
