package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

var rumorTimeout = 10

// RumorMessage struct
type RumorMessage struct {
	Origin string
	ID     uint32
	Text   string
}

// StatusPacket struct
type StatusPacket struct {
	Want []PeerStatus
}

// PeerStatus struct
type PeerStatus struct {
	Identifier string
	NextID     uint32
}

func (gossiper *Gossiper) startRumorMongering(extPacket *ExtendedGossipPacket) {
	peersWithRumor := []*net.UDPAddr{extPacket.SenderAddr}
	peers := gossiper.GetPeersAtomic()
	availablePeers := helpers.DifferenceString(peers, peersWithRumor)
	flipped := false

	if len(availablePeers) != 0 {
		randomPeer := gossiper.getRandomPeer(availablePeers)
		coin := 0
		for coin == 0 {
			statusReceived := gossiper.sendRumorWithTimeout(extPacket.Packet, randomPeer)
			if statusReceived {
				coin = rand.Int() % 2
				flipped = true
			}

			if coin == 0 {
				peers := gossiper.GetPeersAtomic()
				peersWithRumor = []*net.UDPAddr{randomPeer}
				availablePeers := helpers.DifferenceString(peers, peersWithRumor)
				if len(availablePeers) == 0 {
					return
				}
				randomPeer = gossiper.getRandomPeer(availablePeers)

				if flipped {
					fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
					flipped = false
				}
			}
		}
	}
}

func (gossiper *Gossiper) sendRumorWithTimeout(packet *GossipPacket, peer *net.UDPAddr) bool {

	rumorChan := gossiper.createOrGetMongerChannel(peer.String())

	go gossiper.sendPacket(packet, peer)
	fmt.Println("MONGERING with " + peer.String())

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)

	for {
		select {
		case <-rumorChan:
			timer.Stop()
			return true

		case <-timer.C:
			return false
		}
	}
}

func (gossiper *Gossiper) handlePeerStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		go gossiper.notifyMongerChannel(extPacket.SenderAddr.String())

		toSend := gossiper.getPeerStatusOtherNeeds(extPacket.Packet.Status.Want)

		if toSend != nil {
			packetToSend := gossiper.getPacketFromPeerStatus(*toSend)
			gossiper.sendPacket(packetToSend, extPacket.SenderAddr)
		} else {
			wanted := gossiper.getPeerStatusINeed(extPacket.Packet.Status.Want)
			if wanted != nil {
				statusToSend := gossiper.getStatusToSend()
				gossiper.sendPacket(statusToSend, extPacket.SenderAddr)
			} else {
				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
			}
		}
	}
}
