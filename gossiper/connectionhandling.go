package gossiper

import (
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

var maxBufferSize = 1024

func (gossiper *Gossiper) receivePackets(data *NetworkData, channels map[string]chan *ExtendedGossipPacket) {
	for {
		packetFromPeer := &GossipPacket{}
		packetFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)
		_, addr, err := data.Conn.ReadFromUDP(packetBytes)

		helpers.ErrorCheck(err)

		if data.Addr.String() == gossiper.clientData.Addr.String() {
			protobuf.Decode(packetBytes, packetFromClient)
			if gossiper.simpleMode {
				simplePacket := &SimpleMessage{Contents: packetFromClient.Text}
				channels["client"] <- &ExtendedGossipPacket{Packet: &GossipPacket{Simple: simplePacket}, SenderAddr: addr}
			} else {
				rumorPacket := &RumorMessage{Text: packetFromClient.Text}
				channels["client"] <- &ExtendedGossipPacket{Packet: &GossipPacket{Rumor: rumorPacket}, SenderAddr: addr}
			}
		} else {
			protobuf.Decode(packetBytes, packetFromPeer)
			modeType := getTypeMode(packetFromPeer)
			if (modeType == "simple" && gossiper.simpleMode) || (modeType != "simple" && !gossiper.simpleMode) {
				channels[modeType] <- &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
			}
		}
	}
}

func (gossiper *Gossiper) sendPacket(packet *GossipPacket, address *net.UDPAddr) {
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	gossiper.gossiperData.Conn.WriteToUDP(packetToSend, address)
}

func (gossiper *Gossiper) broadcastToPeers(packet *ExtendedGossipPacket) {
	for _, peer := range gossiper.GetPeersAtomic() {
		if peer.String() != packet.SenderAddr.String() {
			gossiper.sendPacket(packet.Packet, peer)
		}
	}
}

func (gossiper *Gossiper) sendStatusPacket(addr *net.UDPAddr) {
	myStatus := gossiper.createStatus()
	statusPacket := &StatusPacket{Want: myStatus}
	packet := &GossipPacket{Status: statusPacket}
	gossiper.sendPacket(packet, addr)
}

func (gossiper *Gossiper) sendPacketFromStatus(toSend []PeerStatus, addr *net.UDPAddr) {
	for _, ps := range toSend {
		packets := gossiper.getPacketsFromStatus(ps)
		for _, m := range packets {
			fmt.Println("MONGERING with " + addr.String())
			gossiper.sendPacket(m, addr)
			return
		}
	}
}
