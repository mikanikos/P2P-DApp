package gossiper

import (
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

var maxBufferSize = 10000

func (gossiper *Gossiper) receivePacketsFromClient(clientChannel chan *helpers.Message) {
	for {
		messageFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)
		_, _, err := gossiper.clientData.Conn.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err)

		protobuf.Decode(packetBytes, messageFromClient)

		clientChannel <- messageFromClient
	}
}

func (gossiper *Gossiper) receivePacketsFromPeers() {
	for {
		packetFromPeer := &GossipPacket{}
		packetBytes := make([]byte, maxBufferSize)
		_, addr, err := gossiper.gossiperData.Conn.ReadFromUDP(packetBytes)

		helpers.ErrorCheck(err)

		protobuf.Decode(packetBytes, packetFromPeer)

		modeType := getTypeFromGossip(packetFromPeer)

		if (modeType == "simple" && gossiper.simpleMode) || (modeType != "simple" && !gossiper.simpleMode) {
			gossiper.channels[modeType] <- &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
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

// func (gossiper *Gossiper) sendStatusPacket(addr *net.UDPAddr) {
// 	myStatus := gossiper.createStatus()
// 	statusPacket := &StatusPacket{Want: myStatus}
// 	packet := &GossipPacket{Status: statusPacket}
// 	gossiper.sendPacket(packet, addr)
// }

// func (gossiper *Gossiper) sendPacketFromStatus(toSend []PeerStatus, addr *net.UDPAddr) {
// 	for _, ps := range toSend {
// 		packets := gossiper.getPacketsFromStatus(ps)
// 		for _, m := range packets {
// 			fmt.Println("MONGERING with " + addr.String())
// 			gossiper.sendPacket(m, addr)
// 			return
// 		}
// 	}
// }
