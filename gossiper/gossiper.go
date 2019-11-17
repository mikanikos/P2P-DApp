package gossiper

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

// Gossiper struct
type Gossiper struct {
	name              string
	channels          map[string]chan *ExtendedGossipPacket
	clientData        *NetworkData
	gossiperData      *NetworkData
	peers             MutexPeers
	origins           MutexOrigins
	originPackets     PacketsStorage
	myStatus          MutexStatus
	originLastID      MutexStatus
	seqID             uint32
	statusChannels    sync.Map
	mongeringChannels sync.Map
	routingTable      MutexRoutingTable
	myFileChunks      sync.Map
	//mySharedFiles      sync.Map
	myFiles            sync.Map
	hashChannels       sync.Map
	filesIndexed       chan *FileMetadata
	filesDownloaded    chan *FileMetadata
	lastSearchRequests MutexSearchResult
}

// NewGossiper function
func NewGossiper(name string, address string, peersList []string, uiPort string, simple bool) *Gossiper {

	simpleMode = simple

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
		origins:           MutexOrigins{Origins: make([]string, 0)},
		originPackets:     PacketsStorage{OriginPacketsMap: sync.Map{}, LatestMessages: make(chan *RumorMessage, latestMessagesBuffer)},
		myStatus:          MutexStatus{Status: make(map[string]uint32)},
		originLastID:      MutexStatus{Status: make(map[string]uint32)},
		seqID:             1,
		statusChannels:    sync.Map{},
		mongeringChannels: sync.Map{},
		routingTable:      MutexRoutingTable{RoutingTable: make(map[string]*net.UDPAddr)},
		myFileChunks:      sync.Map{},
		//mySharedFiles:      sync.Map{},
		myFiles:            sync.Map{},
		hashChannels:       sync.Map{},
		filesIndexed:       make(chan *FileMetadata, 3),
		filesDownloaded:    make(chan *FileMetadata, 3),
		lastSearchRequests: MutexSearchResult{SearchResults: make(map[string]ExtendedSearchRequest)},
	}
}

// Run method
func (gossiper *Gossiper) Run() {

	rand.Seed(time.Now().UnixNano())

	initializeDirectories()

	clientChannel := make(chan *helpers.Message)
	go gossiper.processClientMessages(clientChannel)

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
	}

	go gossiper.receivePacketsFromClient(clientChannel)
	gossiper.receivePacketsFromPeers()
}

func (gossiper *Gossiper) processSearchRequest() {
	for extPacket := range gossiper.channels["searchRequest"] {

		if !gossiper.isRecentSearchRequest(extPacket.Packet.SearchRequest) {

			go gossiper.sendMatchingLocalFiles(extPacket)
		} else {
			if debug {
				fmt.Println("Too recent request!!!!")
			}
		}
	}
}

func (gossiper *Gossiper) processSearchReply() {
	for extPacket := range gossiper.channels["searchReply"] {

		if extPacket.Packet.SearchReply.Destination == gossiper.name {

			searchResults := extPacket.Packet.SearchReply.Results

			for _, res := range searchResults {

				printSearchMatchMessage(extPacket.Packet.SearchReply.Origin, res)

				fileData := &SearchResult{FileName: res.FileName, MetafileHash: res.MetafileHash, ChunkCount: res.ChunkCount, ChunkMap: make([]uint64, 0)}
				value, loaded := gossiper.myFiles.LoadOrStore(hex.EncodeToString(res.MetafileHash), &FileMetadata{FileSearchData: fileData})
				fileMetadata := value.(*FileMetadata)

				// WHAT TO DO IF FILE WAS LOADED, I.E. ALREADY PRESENT FROM PREVIOUS SEARCH OR PREVIOUS DOWNLOAD? PROBABLY NOTHING...

				if !loaded {
					gossiper.downloadMetafile(extPacket.Packet.SearchReply.Origin, fileMetadata)
				}

				gossiper.storeChunksOwner(extPacket.Packet.SearchReply.Origin, res.ChunkMap, fileMetadata)
			}
		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet)
		}

	}
}

func (gossiper *Gossiper) processDataRequest() {
	for extPacket := range gossiper.channels["dataRequest"] {

		if debug {
			fmt.Println("Got data request")
		}

		if extPacket.Packet.DataRequest.Destination == gossiper.name {

			keyHash := hex.EncodeToString(extPacket.Packet.DataRequest.HashValue)
			packetToSend := &GossipPacket{DataReply: &DataReply{Origin: gossiper.name, Destination: extPacket.Packet.DataRequest.Origin, HopLimit: uint32(hopLimit), HashValue: extPacket.Packet.DataRequest.HashValue}}

			// try loading from metafiles
			fileValue, loaded := gossiper.myFiles.Load(keyHash)

			if loaded {

				fileRequested := fileValue.(*FileMetadata)
				packetToSend.DataReply.Data = *fileRequested.MetaFile

				if debug {
					fmt.Println("Sent metafile")
				}
				go gossiper.forwardPrivateMessage(packetToSend)

			} else {

				// try loading from chunks
				chunkData, loaded := gossiper.myFileChunks.Load(keyHash)

				if loaded {

					chunkRequested := chunkData.(*[]byte)
					packetToSend.DataReply.Data = *chunkRequested

					if debug {
						fmt.Println("Sent chunk " + keyHash + " to " + packetToSend.DataReply.Destination)
					}
					go gossiper.forwardPrivateMessage(packetToSend)
				} else {
					packetToSend.DataReply.Data = nil
					go gossiper.forwardPrivateMessage(packetToSend)
				}
			}
		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet)
		}
	}
}

func (gossiper *Gossiper) processDataReply() {
	for extPacket := range gossiper.channels["dataReply"] {

		if debug {
			fmt.Println("Got data reply")
		}

		if extPacket.Packet.DataReply.Destination == gossiper.name {

			if extPacket.Packet.DataReply.Data != nil {

				validated := checkHash(extPacket.Packet.DataReply.HashValue, extPacket.Packet.DataReply.Data)

				if validated {
					value, loaded := gossiper.hashChannels.Load(hex.EncodeToString(extPacket.Packet.DataReply.HashValue) + extPacket.Packet.DataReply.Origin)

					if debug {
						fmt.Println("Found channel?")
					}

					if loaded {
						channel := value.(chan *DataReply)
						go func(c chan *DataReply, d *DataReply) {
							c <- d
						}(channel, extPacket.Packet.DataReply)
					}
				}
			}

		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet)
		}
	}
}

func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range gossiper.channels["private"] {

		if extPacket.Packet.Private.Destination == gossiper.name {
			if hw2 {
				gossiper.printPeerMessage(extPacket)
			}
			go func(p *PrivateMessage) {
				gossiper.originPackets.LatestMessages <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(extPacket.Packet.Private)

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
			if hw1 {
				gossiper.printClientMessage(message)
			}

			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.name, RelayPeerAddr: gossiper.gossiperData.Addr.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.broadcastToPeers(packet)

		case "private":
			if hw2 {
				gossiper.printClientMessage(message)
			}

			privatePacket := &PrivateMessage{Origin: gossiper.name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go func(p *PrivateMessage) {
				gossiper.originPackets.LatestMessages <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(privatePacket)

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

		case "dataRequest":

			go gossiper.requestFile(*message.File, *message.Request)

		case "searchRequest":

			keywordsSplitted := helpers.RemoveDuplicatesFromSlice(strings.Split(*message.Keywords, ","))
			requestPacket := &SearchRequest{Origin: gossiper.name, Budget: *message.Budget, Keywords: keywordsSplitted}
			packet.Packet = &GossipPacket{SearchRequest: requestPacket}

			go gossiper.searchFilePeriodically(packet)

		default:
			if debug {
				fmt.Println("Unkown packet!")
			}
		}
	}
}

func (gossiper *Gossiper) processSimpleMessages() {
	for extPacket := range gossiper.channels["simple"] {

		if hw1 {
			gossiper.printPeerMessage(extPacket)
		}

		extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Addr.String()

		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) processRumorMessages() {
	for extPacket := range gossiper.channels["rumor"] {

		gossiper.printPeerMessage(extPacket)

		gossiper.updateRoutingTable(extPacket)

		isMessageKnown := gossiper.addMessage(extPacket)

		// send status
		statusToSend := gossiper.getStatusToSend()
		gossiper.sendPacket(statusToSend, extPacket.SenderAddr)

		if !isMessageKnown {
			go gossiper.startRumorMongering(extPacket)
		}
	}
}

func (gossiper *Gossiper) processStatusMessages() {
	for extPacket := range gossiper.channels["status"] {

		if hw1 {
			gossiper.printStatusMessage(extPacket)
		}

		value, exists := gossiper.statusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket))
		channelPeer := value.(chan *ExtendedGossipPacket)
		if !exists {
			go gossiper.handlePeerStatus(channelPeer)
		}
		go func(c chan *ExtendedGossipPacket, p *ExtendedGossipPacket) {
			c <- p
		}(channelPeer, extPacket)

	}
}
