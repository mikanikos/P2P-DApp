package gossiper

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) processTLCMessage() {
	for extPacket := range gossiper.channels["tlcMes"] {

		gossiper.updateRoutingTable(extPacket.Packet.TLCMessage.Origin, "", extPacket.Packet.TLCMessage.ID, extPacket.SenderAddr)

		//isMessageKnown := gossiper.addMessage(extPacket.Packet)

		// if isTXValid(extPacket.Packet.TLCMessage.TxBlock) {

		// }

		statusToSend := gossiper.getStatusToSend()
		gossiper.sendPacket(statusToSend, extPacket.SenderAddr)

		// if !isMessageKnown {
		// 	go gossiper.startRumorMongering(extPacket)
		// }

		// Ack message
		privatePacket := &TLCAck{Origin: gossiper.name, ID: extPacket.Packet.TLCMessage.ID, Destination: extPacket.Packet.TLCMessage.Origin, HopLimit: uint32(hopLimit)}
		go gossiper.forwardPrivateMessage(&GossipPacket{Ack: privatePacket}, &privatePacket.HopLimit, privatePacket.Destination)
	}
}

func (gossiper *Gossiper) processTLCAck() {
	for extPacket := range gossiper.channels["tlcAck"] {

		if extPacket.Packet.Ack.Destination == gossiper.name {
			if hw2 {
				printPeerMessage(extPacket, gossiper.GetPeersAtomic())
			}

		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Ack.HopLimit, extPacket.Packet.Ack.Destination)
		}
	}
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

				value, loaded := gossiper.fileHandler.myFiles.LoadOrStore(hex.EncodeToString(res.MetafileHash), &FileMetadata{FileName: res.FileName, MetafileHash: res.MetafileHash, ChunkCount: res.ChunkCount, ChunkMap: make([]uint64, 0)})
				gossiper.fileHandler.filesList.LoadOrStore(hex.EncodeToString(res.MetafileHash)+res.FileName, &FileIDPair{FileName: res.FileName, EncMetaHash: hex.EncodeToString(res.MetafileHash)})
				fileMetadata := value.(*FileMetadata)

				if !loaded {
					gossiper.downloadMetafile(extPacket.Packet.SearchReply.Origin, fileMetadata)
				}

				gossiper.storeChunksOwner(extPacket.Packet.SearchReply.Origin, res.ChunkMap, fileMetadata)

				gossiper.addSearchFileForGUI(fileMetadata)
			}
		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.SearchReply.HopLimit, extPacket.Packet.SearchReply.Destination)
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
			fileValue, loaded := gossiper.fileHandler.myFiles.Load(keyHash)

			if loaded {

				fileRequested := fileValue.(*FileMetadata)
				packetToSend.DataReply.Data = *fileRequested.MetaFile

				if debug {
					fmt.Println("Sent metafile")
				}

			} else {

				// try loading from chunks
				chunkData, loaded := gossiper.fileHandler.myFileChunks.Load(keyHash)

				if loaded {

					chunkRequested := chunkData.(*ChunkOwners)
					packetToSend.DataReply.Data = *chunkRequested.Data

					if debug {
						fmt.Println("Sent chunk " + keyHash + " to " + packetToSend.DataReply.Destination)
					}
				} else {
					packetToSend.DataReply.Data = nil
				}
			}
			go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.DataReply.HopLimit, packetToSend.DataReply.Destination)
		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.DataRequest.HopLimit, extPacket.Packet.DataRequest.Destination)
		}
	}
}

func (gossiper *Gossiper) processDataReply() {
	for extPacket := range gossiper.channels["dataReply"] {

		if debug {
			fmt.Println("Got data reply")
		}

		if extPacket.Packet.DataReply.Destination == gossiper.name {

			if extPacket.Packet.DataReply.Data != nil && checkHash(extPacket.Packet.DataReply.HashValue, extPacket.Packet.DataReply.Data) {
				value, loaded := gossiper.fileHandler.hashChannels.Load(hex.EncodeToString(extPacket.Packet.DataReply.HashValue) + extPacket.Packet.DataReply.Origin)

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

		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.DataReply.HopLimit, extPacket.Packet.DataReply.Destination)
		}
	}
}

func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range gossiper.channels["private"] {

		if extPacket.Packet.Private.Destination == gossiper.name {
			if hw2 {
				printPeerMessage(extPacket, gossiper.GetPeersAtomic())
			}
			go func(p *PrivateMessage) {
				gossiper.uiHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(extPacket.Packet.Private)

		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Private.HopLimit, extPacket.Packet.Private.Destination)
		}
	}
}

func (gossiper *Gossiper) processClientMessages(clientChannel chan *helpers.Message) {
	for message := range clientChannel {

		packet := &ExtendedGossipPacket{SenderAddr: gossiper.gossiperData.Addr}

		switch typeMessage := getTypeFromMessage(message); typeMessage {

		case "simple":
			if hw1 {
				printClientMessage(message, gossiper.GetPeersAtomic())
			}

			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.name, RelayPeerAddr: gossiper.gossiperData.Addr.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.broadcastToPeers(packet)

		case "private":
			if hw2 {
				printClientMessage(message, gossiper.GetPeersAtomic())
			}

			privatePacket := &PrivateMessage{Origin: gossiper.name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go func(p *PrivateMessage) {
				gossiper.uiHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(privatePacket)

			go gossiper.forwardPrivateMessage(packet.Packet, &packet.Packet.Private.HopLimit, packet.Packet.Private.Destination)

		case "rumor":
			printClientMessage(message, gossiper.GetPeersAtomic())

			id := atomic.LoadUint32(&gossiper.seqID)
			atomic.AddUint32(&gossiper.seqID, uint32(1))
			rumorPacket := &RumorMessage{ID: id, Origin: gossiper.name, Text: message.Text}
			packet.Packet = &GossipPacket{Rumor: rumorPacket}

			gossiper.storeRumorMessage(packet.Packet.Rumor)
			go gossiper.startRumorMongering(packet)

		case "file":
			go gossiper.indexFile(message.File)

		case "dataRequest":
			go gossiper.requestFile(*message.File, *message.Request, *message.Destination)

		case "searchRequest":

			keywordsSplitted := helpers.RemoveDuplicatesFromSlice(strings.Split(*message.Keywords, ","))

			requestPacket := &SearchRequest{Origin: gossiper.name, Keywords: keywordsSplitted}
			packet.Packet = &GossipPacket{SearchRequest: requestPacket}

			if *message.Budget != 0 {
				requestPacket.Budget = *message.Budget
				go gossiper.searchFilePeriodically(packet, false)
			} else {
				requestPacket.Budget = uint64(2)
				go gossiper.searchFilePeriodically(packet, true)
			}

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
			printPeerMessage(extPacket, gossiper.GetPeersAtomic())
		}

		extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Addr.String()

		go gossiper.broadcastToPeers(extPacket)
	}
}

func (gossiper *Gossiper) processRumorMessages() {
	for extPacket := range gossiper.channels["rumor"] {

		printPeerMessage(extPacket, gossiper.GetPeersAtomic())

		gossiper.updateRoutingTable(extPacket.Packet.Rumor.Origin, extPacket.Packet.Rumor.Text, extPacket.Packet.Rumor.ID, extPacket.SenderAddr)

		isMessageKnown := gossiper.storeRumorMessage(extPacket.Packet.Rumor)

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
			printStatusMessage(extPacket, gossiper.GetPeersAtomic())
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
