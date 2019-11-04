package gossiper

import (
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) receivePacketsFromClient(clientChannel chan *helpers.Message) {
	for {
		messageFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)
		//fmt.Println("Waiting")

		n, _, err := gossiper.clientData.Conn.ReadFromUDP(packetBytes)
		//fmt.Println("Got packet from client")
		helpers.ErrorCheck(err)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
		}

		protobuf.Decode(packetBytes[:n], messageFromClient)
		helpers.ErrorCheck(err)

		//fmt.Println("got from client")

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
		}

		gossiper.AddPeer(addr)

		err = protobuf.Decode(packetBytes[:n], packetFromPeer)
		helpers.ErrorCheck(err)

		//fmt.Println("got from peer")

		modeType := getTypeFromGossip(packetFromPeer)

		if (modeType == "simple" && gossiper.simpleMode) || (modeType != "simple" && !gossiper.simpleMode) {
			packet := &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
			go func(p *ExtendedGossipPacket) {
				gossiper.channels[modeType] <- p
			}(packet)
		} else {
			//fmt.Println("ERROR: message can't be accepted in this operation mode")
		}
	}
}

func (gossiper *Gossiper) sendPacket(packet *GossipPacket, address *net.UDPAddr) {
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err)
	_, err = gossiper.gossiperData.Conn.WriteToUDP(packetToSend, address)
	helpers.ErrorCheck(err)
	//fmt.Println("Send message to " + address.String())
}

func (gossiper *Gossiper) broadcastToPeers(packet *ExtendedGossipPacket) {
	for _, peer := range gossiper.GetPeersAtomic() {
		if peer.String() != packet.SenderAddr.String() {
			//fmt.Println("to " + peer.String())
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
