package gossiper

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mikanikos/Peerster/helpers"
)

// process tlc message
func (gossiper *Gossiper) processTLCMessage() {
	for extPacket := range gossiper.packetChannels["tlcMes"] {

		// handle gossip message
		gossiper.handleGossipMessage(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

		// handle tlc message
		if hw3ex2Mode || hw3ex3Mode || hw3ex4Mode {
			go gossiper.handleTLCMessage(extPacket)
		}
	}
}

// process tlc acks
func (gossiper *Gossiper) processTLCAck() {
	for extPacket := range gossiper.packetChannels["tlcAck"] {
		if extPacket.Packet.Ack.Destination == gossiper.name {

			if debug {
				fmt.Println("Got ack for " + fmt.Sprint(extPacket.Packet.Ack.ID) + " from " + extPacket.Packet.Ack.Origin)
			}

			// send to the ack channel
			go func(v *TLCAck) {
				gossiper.blockchainHandler.tlcAckChan <- v
			}(extPacket.Packet.Ack)
		} else {
			// forward tlc ack to peers
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Ack.HopLimit, extPacket.Packet.Ack.Destination)
		}
	}
}

// process search request
func (gossiper *Gossiper) processSearchRequest() {
	for extPacket := range gossiper.packetChannels["searchRequest"] {

		// check if search request is not recent
		if !gossiper.isRecentSearchRequest(extPacket.Packet.SearchRequest) {
			// send matching local files
			go gossiper.sendMatchingLocalFiles(extPacket)
		} else {
			if debug {
				fmt.Println("Too recent request!!!!")
			}
		}
	}
}

// process search replies
func (gossiper *Gossiper) processSearchReply() {
	for extPacket := range gossiper.packetChannels["searchReply"] {

		// if it's for me, I handle search results
		if extPacket.Packet.SearchReply.Destination == gossiper.name {

			searchResults := extPacket.Packet.SearchReply.Results
			for _, res := range searchResults {
				gossiper.handleSearchResult(extPacket.Packet.SearchReply.Origin, res)
			}
		} else {
			// if not for me, I forward the reply
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.SearchReply.HopLimit, extPacket.Packet.SearchReply.Destination)
		}
	}
}

// process data request
func (gossiper *Gossiper) processDataRequest() {
	for extPacket := range gossiper.packetChannels["dataRequest"] {

		if debug {
			fmt.Println("Got data request")
		}

		// if for me, I I handle the data request
		if extPacket.Packet.DataRequest.Destination == gossiper.name {

			keyHash := hex.EncodeToString(extPacket.Packet.DataRequest.HashValue)
			packetToSend := &GossipPacket{DataReply: &DataReply{Origin: gossiper.name, Destination: extPacket.Packet.DataRequest.Origin, HopLimit: uint32(hopLimit), HashValue: extPacket.Packet.DataRequest.HashValue}}

			// try checking hash from metafiles
			fileValue, loaded := gossiper.fileHandler.myFiles.Load(keyHash)

			if loaded {

				fileRequested := fileValue.(*FileMetadata)
				packetToSend.DataReply.Data = *fileRequested.MetaFile

				if debug {
					fmt.Println("Sent metafile")
				}

			} else {

				// try checking hash from chunks
				chunkData, loaded := gossiper.fileHandler.myFileChunks.Load(keyHash)

				if loaded {

					chunkRequested := chunkData.(*ChunkOwners)
					packetToSend.DataReply.Data = *chunkRequested.Data

					if debug {
						fmt.Println("Sent chunk " + keyHash + " to " + packetToSend.DataReply.Destination)
					}
				} else {
					// no match, send back nil
					packetToSend.DataReply.Data = nil
				}
			}
			go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.DataReply.HopLimit, packetToSend.DataReply.Destination)
		} else {
			// if not for me, I forward the request
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.DataRequest.HopLimit, extPacket.Packet.DataRequest.Destination)
		}
	}
}

// process data reply
func (gossiper *Gossiper) processDataReply() {
	for extPacket := range gossiper.packetChannels["dataReply"] {

		if debug {
			fmt.Println("Got data reply")
		}

		// if for me, handle data reply
		if extPacket.Packet.DataReply.Destination == gossiper.name {

			// check integrity of the hash
			if extPacket.Packet.DataReply.Data != nil && checkHash(extPacket.Packet.DataReply.HashValue, extPacket.Packet.DataReply.Data) {
				value, loaded := gossiper.fileHandler.hashChannels.Load(hex.EncodeToString(extPacket.Packet.DataReply.HashValue) + extPacket.Packet.DataReply.Origin)

				// send it to the appropriate channel
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

// process private message
func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range gossiper.packetChannels["private"] {

		// if for me, handle private message
		if extPacket.Packet.Private.Destination == gossiper.name {
			if hw2 {
				gossiper.printPeerMessage(extPacket, gossiper.GetPeersAtomic())
			}
			// send it to gui
			go func(p *PrivateMessage) {
				gossiper.uiHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(extPacket.Packet.Private)

		} else {
			// if not for me, forward message
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Private.HopLimit, extPacket.Packet.Private.Destination)
		}
	}
}

// process client messages
func (gossiper *Gossiper) processClientMessages(clientChannel chan *helpers.Message) {
	for message := range clientChannel {

		packet := &ExtendedGossipPacket{SenderAddr: gossiper.gossiperData.Address}

		// get type of the message
		switch typeMessage := getTypeFromMessage(message); typeMessage {

		case "simple":
			if hw1 {
				printClientMessage(message, gossiper.GetPeersAtomic())
			}

			// create simple packet and broadcast
			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.name, RelayPeerAddr: gossiper.gossiperData.Address.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.broadcastToPeers(packet)

		case "private":
			if hw2 {
				printClientMessage(message, gossiper.GetPeersAtomic())
			}

			// create private message and forward it
			privatePacket := &PrivateMessage{Origin: gossiper.name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go func(p *PrivateMessage) {
				gossiper.uiHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(privatePacket)

			go gossiper.forwardPrivateMessage(packet.Packet, &packet.Packet.Private.HopLimit, packet.Packet.Private.Destination)

		case "rumor":
			printClientMessage(message, gossiper.GetPeersAtomic())

			// create rumor
			extPacket := gossiper.createRumorMessage(message.Text)

			// rumor monger it
			go gossiper.startRumorMongering(extPacket, gossiper.name, extPacket.Packet.Rumor.ID)

		case "file":
			gossiper.indexFile(message.File)

		case "dataRequest":
			go gossiper.requestFile(*message.File, *message.Request, *message.Destination)

		case "searchRequest":

			// create search request packet and handle it
			keywordsSplitted := helpers.RemoveDuplicatesFromSlice(strings.Split(*message.Keywords, ","))

			requestPacket := &SearchRequest{Origin: gossiper.name, Keywords: keywordsSplitted, Budget: *message.Budget}
			packet.Packet = &GossipPacket{SearchRequest: requestPacket}

			needIncrement := (*message.Budget == 0)

			if needIncrement {
				requestPacket.Budget = uint64(defaultBudget)
			}

			go gossiper.searchFilePeriodically(packet, needIncrement)

		default:
			if debug {
				fmt.Println("Unkown packet!")
			}
		}
	}
}

// process simple message
func (gossiper *Gossiper) processSimpleMessages() {
	for extPacket := range gossiper.packetChannels["simple"] {

		if hw1 {
			gossiper.printPeerMessage(extPacket, gossiper.GetPeersAtomic())
		}

		// set relay address and broadcast
		extPacket.Packet.Simple.RelayPeerAddr = gossiper.gossiperData.Address.String()

		go gossiper.broadcastToPeers(extPacket)
	}
}

// process rumor message
func (gossiper *Gossiper) processRumorMessages() {
	for extPacket := range gossiper.packetChannels["rumor"] {

		// handle gossip message
		gossiper.handleGossipMessage(extPacket, extPacket.Packet.Rumor.Origin, extPacket.Packet.Rumor.ID)
	}
}

// process status message
func (gossiper *Gossiper) processStatusMessages() {
	for extPacket := range gossiper.packetChannels["status"] {

		if hw1 {
			printStatusMessage(extPacket, gossiper.GetPeersAtomic())
		}

		// get status channel for peer and send it there
		value, exists := gossiper.gossipHandler.StatusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket, maxChannelSize))
		channelPeer := value.(chan *ExtendedGossipPacket)
		if !exists {
			go gossiper.handlePeerStatus(channelPeer)
		}
		go func(c chan *ExtendedGossipPacket, p *ExtendedGossipPacket) {
			c <- p
		}(channelPeer, extPacket)

	}
}
