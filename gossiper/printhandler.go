package gossiper

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/mikanikos/Peerster/helpers"
)

func printStatusMessage(extPacket *ExtendedGossipPacket, peers []*net.UDPAddr) {
	message := "STATUS from " + extPacket.SenderAddr.String() + " "
	for _, value := range extPacket.Packet.Status.Want {
		message = message + "peer " + value.Identifier + " nextID " + fmt.Sprint(value.NextID) + " "
	}
	if hw1 {
		fmt.Println(message[:len(message)-1])
		printPeers(peers)
	}
}

func printSearchMatchMessage(origin string, res *SearchResult) {
	message := "FOUND match " + res.FileName + " at " + origin + " metafile=" + hex.EncodeToString(res.MetafileHash) + " chunks="
	for _, elem := range res.ChunkMap {
		message = message + fmt.Sprint(elem) + ","
	}
	if hw3 {
		fmt.Println(message[:len(message)-1])
	}
}

func printPeerMessage(extPacket *ExtendedGossipPacket, peers []*net.UDPAddr) {
	if simpleMode {
		if hw1 {
			fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)
		}
	} else {
		if extPacket.Packet.Private != nil {
			if hw2 {
				fmt.Println("PRIVATE origin " + extPacket.Packet.Private.Origin + " hop-limit " + fmt.Sprint(extPacket.Packet.Private.HopLimit) + " contents " + extPacket.Packet.Private.Text)
			}
		} else {
			fmt.Println("RUMOR origin " + extPacket.Packet.Rumor.Origin + " from " + extPacket.SenderAddr.String() + " ID " + fmt.Sprint(extPacket.Packet.Rumor.ID) + " contents " + extPacket.Packet.Rumor.Text)
		}
	}
	if hw1 {
		printPeers(peers)
	}
}

func printClientMessage(message *helpers.Message, peers []*net.UDPAddr) {
	if message.Destination != nil {
		fmt.Println("CLIENT MESSAGE " + message.Text + " dest " + *message.Destination)
	} else {
		fmt.Println("CLIENT MESSAGE " + message.Text)
	}
	if hw1 {
		printPeers(peers)
	}
}

func printPeers(peers []*net.UDPAddr) {
	listPeers := helpers.GetArrayStringFromAddresses(peers)
	if hw1 {
		fmt.Println("PEERS " + strings.Join(listPeers, ","))
	}
}

func printTLCMessage(tlc *TLCMessage) {
	messageToPrint := "GOSSIP origin " + tlc.Origin +
		" ID " + fmt.Sprint(tlc.ID) +
		" filename " + tlc.TxBlock.Transaction.Name +
		" size " + fmt.Sprint(tlc.TxBlock.Transaction.Size) +
		" metahash " + hex.EncodeToString(tlc.TxBlock.Transaction.MetafileHash)

	if tlc.Confirmed > -1 {
		fmt.Println("CONFIRMED " + messageToPrint)
	} else {
		fmt.Println("UNCONFIRMED " + messageToPrint)
	}
}

func printRoundMessage(round uint32, confirmations map[string]uint32) {
	message := "ADVANCING TO round " + fmt.Sprint(round) + " BASED ON CONFIRMED MESSAGES "

	i := 1
	for key, value := range confirmations {
		message = message + "origin" + fmt.Sprint(i) + " " + key + " ID" + fmt.Sprint(i) + " " + fmt.Sprint(value) + ", "
		i++
	}
	if hw3ex3Mode {
		fmt.Println(message[:len(message)-2])
	}
}
