package gossiper

import (
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

// ExtendedGossipPacket struct: gossip packet + address of the sender 
type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

// NetworkData struct: connection + address
type NetworkData struct {
	Conn *net.UDPConn
	Addr *net.UDPAddr
}

// process incoming packets from client and send them to the client channel for further processing
func (gossiper *Gossiper) receivePacketsFromClient(clientChannel chan *helpers.Message) {
	for {
		messageFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)

		// read from socket
		n, _, err := gossiper.clientData.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		// decode message
		protobuf.Decode(packetBytes[:n], messageFromClient)
		helpers.ErrorCheck(err)

		// send it to channel
		go func(m *helpers.Message) {
			clientChannel <- m
		}(messageFromClient)
	}
}

// process incoming packets from other peers and send them dynamically to the appropriate channel for further processing
func (gossiper *Gossiper) receivePacketsFromPeers() {
	for {
		packetFromPeer := &GossipPacket{}
		packetBytes := make([]byte, maxBufferSize)

		// read from socket
		n, addr, err := gossiper.gossiperData.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		// add peer
		gossiper.AddPeer(addr)

		// decode message
		err = protobuf.Decode(packetBytes[:n], packetFromPeer)
		helpers.ErrorCheck(err)

		// get type of message and send it dynamically to the correct channel
		modeType := getTypeFromGossip(packetFromPeer)

		if modeType != "unknwon" {
			if (modeType == "simple" && simpleMode) || (modeType != "simple" && !simpleMode) {
				packet := &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
				go func(p *ExtendedGossipPacket) {
					gossiper.packetChannels[modeType] <- p
				}(packet)
			} else {
				fmt.Println("ERROR: message can't be accepted in this operation mode")
			}
		}
	}
}

// send given packet to the address specified
func (gossiper *Gossiper) sendPacket(packet *GossipPacket, address *net.UDPAddr) {
	// encode message
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	
	// send message
	_, err = gossiper.gossiperData.Conn.WriteToUDP(packetToSend, address)
	helpers.ErrorCheck(err)
}

// broadcast message to all the know peers
func (gossiper *Gossiper) broadcastToPeers(packet *ExtendedGossipPacket) {
	peers := gossiper.GetPeersAtomic()
	for _, peer := range peers {
		if peer.String() != packet.SenderAddr.String() {
			gossiper.sendPacket(packet.Packet, peer)
		}
	}
}
