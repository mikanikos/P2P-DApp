package gossiper

import (
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

// ExtendedGossipPacket struct
type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

func (gossiper *Gossiper) receivePacketsFromClient(clientChannel chan *helpers.Message) {
	for {
		messageFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)

		n, _, err := gossiper.clientData.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		protobuf.Decode(packetBytes[:n], messageFromClient)
		helpers.ErrorCheck(err)

		go func(m *helpers.Message) {
			clientChannel <- m
		}(messageFromClient)
	}
}

func (gossiper *Gossiper) receivePacketsFromPeers() {
	for {
		packetFromPeer := &GossipPacket{}
		packetBytes := make([]byte, maxBufferSize)
		n, addr, err := gossiper.gossiperData.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		gossiper.AddPeer(addr)

		err = protobuf.Decode(packetBytes[:n], packetFromPeer)
		helpers.ErrorCheck(err)

		modeType := getTypeFromGossip(packetFromPeer)

		if (modeType == "simple" && gossiper.simpleMode) || (modeType != "simple" && !gossiper.simpleMode) {
			packet := &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
			go func(p *ExtendedGossipPacket) {
				gossiper.channels[modeType] <- p
			}(packet)
		} else {
			fmt.Println("ERROR: message can't be accepted in this operation mode")
		}
	}
}

func (gossiper *Gossiper) sendPacket(packet *GossipPacket, address *net.UDPAddr) {
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	_, err = gossiper.gossiperData.Conn.WriteToUDP(packetToSend, address)
	helpers.ErrorCheck(err)
}

func (gossiper *Gossiper) broadcastToPeers(packet *ExtendedGossipPacket) {
	peers := gossiper.GetPeersAtomic()
	for _, peer := range peers {
		if peer.String() != packet.SenderAddr.String() {
			gossiper.sendPacket(packet.Packet, peer)
		}
	}
}
