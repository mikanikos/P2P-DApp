package gossiper

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

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
	mongeringChannels  sync.Map //MutexDummyChannel
	antiEntropyTimeout int
	routingTable       MutexRoutingTable
	routeTimer         int
	myFileChunks       sync.Map
	mySharedFiles      sync.Map
	myDownloadedFiles  sync.Map
	hashChannels       sync.Map
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
		mongeringChannels:  sync.Map{}, //MutexDummyChannel{Channels: make(map[string]chan bool)},
		antiEntropyTimeout: antiEntropyTimeout,
		routingTable:       MutexRoutingTable{RoutingTable: make(map[string]*net.UDPAddr)},
		routeTimer:         rtimer,
		myFileChunks:       sync.Map{},
		mySharedFiles:      sync.Map{},
		myDownloadedFiles:  sync.Map{},
		hashChannels:       sync.Map{},
	}
}

// Run method
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	initializeDirectories()

	clientChannel := make(chan *helpers.Message)
	go gossiper.processClientMessages(clientChannel)

	if gossiper.simpleMode {
		go gossiper.processSimpleMessages()
	} else {
		go gossiper.processStatusMessages()
		go gossiper.processRumorMessages()
		go gossiper.processPrivateMessages()
		go gossiper.processDataRequest()
		go gossiper.processDataReply()
		go gossiper.startAntiEntropy()
		go gossiper.startRouteRumormongering()
	}

	go gossiper.receivePacketsFromClient(clientChannel)
	gossiper.receivePacketsFromPeers()
}

func (gossiper *Gossiper) processDataRequest() {
	for extPacket := range gossiper.channels["request"] {

		fmt.Println("Got data request")

		if extPacket.Packet.DataRequest.Destination == gossiper.name {

			keyHash := hex.EncodeToString(extPacket.Packet.DataRequest.HashValue)

			packetToSend := &GossipPacket{DataReply: &DataReply{Origin: gossiper.name, Destination: extPacket.Packet.DataRequest.Origin, HopLimit: uint32(hopLimit), HashValue: extPacket.Packet.DataRequest.HashValue}}

			// try loading from metafiles
			fileValue, loaded := gossiper.mySharedFiles.Load(keyHash)

			if loaded {

				fileRequested := fileValue.(*FileMetadata)
				packetToSend.DataReply.Data = *fileRequested.MetaFile

				// fmt.Println(*fileRequested.MetaFile)

				fmt.Println("Sent metafile")
				go gossiper.forwardDataReply(packetToSend)

			} else {

				// try loading from chunks
				chunkData, loaded := gossiper.myFileChunks.Load(keyHash)

				if loaded {

					chunkRequested := chunkData.(*[]byte)
					packetToSend.DataReply.Data = *chunkRequested

					fmt.Println("Sent chunk " + keyHash + " to " + packetToSend.DataReply.Destination)
					go gossiper.forwardDataReply(packetToSend)
				} else {
					packetToSend.DataReply.Data = nil
					go gossiper.forwardDataReply(packetToSend)
				}
			}

		} else {
			go gossiper.forwardDataRequest(extPacket.Packet)
		}
	}
}

func (gossiper *Gossiper) processDataReply() {
	for extPacket := range gossiper.channels["reply"] {

		fmt.Println("Got data reply")

		if extPacket.Packet.DataReply.Destination == gossiper.name {

			// check if I requested this metafile/chunk
			//metadata, loadedMeta := gossiper.myDownloadedFiles.Load(extPacket.Packet.DataReply.HashValue)

			//fileStruct, loadedChunk := gossiper.chunksToFile.Load(extPacket.Packet.DataReply.HashValue)

			// check hash correspond to the data sent

			if extPacket.Packet.DataReply.Data != nil {
				validated := checkHash(extPacket.Packet.DataReply.HashValue, extPacket.Packet.DataReply.Data)

				if validated {
					fmt.Println(hex.EncodeToString(extPacket.Packet.DataReply.HashValue))
					value, loaded := gossiper.hashChannels.Load(hex.EncodeToString(extPacket.Packet.DataReply.HashValue) + extPacket.Packet.DataReply.Origin)

					if loaded {
						channel := value.(chan *DataReply)
						channel <- extPacket.Packet.DataReply
					}
				}
			}

		} else {
			go gossiper.forwardDataReply(extPacket.Packet)
		}
	}
}

func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range gossiper.channels["private"] {

		if extPacket.Packet.Private.Destination == gossiper.name {
			fmt.Println("PRIVATE origin " + extPacket.Packet.Private.Origin + " hop-limit " + fmt.Sprint(extPacket.Packet.Private.HopLimit) + " contents " + extPacket.Packet.Private.Text)
		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet)
		}
	}
}

func (gossiper *Gossiper) processClientMessages(clientChannel chan *helpers.Message) {
	for message := range clientChannel {

		packet := &ExtendedGossipPacket{SenderAddr: gossiper.gossiperData.Addr}

		switch typeMessage := gossiper.getTypeFromMessage(message); typeMessage {

		case "simple":
			gossiper.printClientMessage(message)

			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.name, RelayPeerAddr: gossiper.gossiperData.Addr.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.broadcastToPeers(packet)

		case "private":
			gossiper.printClientMessage(message)

			privatePacket := &PrivateMessage{Origin: gossiper.name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go gossiper.forwardPrivateMessage(packet.Packet)

		case "rumor":
			gossiper.printClientMessage(message)

			id := atomic.LoadUint32(&gossiper.seqID)
			atomic.AddUint32(&gossiper.seqID, uint32(1))
			rumorPacket := &RumorMessage{ID: id, Origin: gossiper.name, Text: message.Text}
			packet.Packet = &GossipPacket{Rumor: rumorPacket}

			gossiper.addMessage(packet)
			go gossiper.startRumorMongering(packet)

		case "file":
			go gossiper.indexFile(message.File)

		case "request":
			requestPacket := &DataRequest{Origin: gossiper.name, Destination: *message.Destination, HashValue: *message.Request, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{DataRequest: requestPacket}

			go gossiper.requestFile(*message.File, packet.Packet)

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
