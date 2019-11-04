package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) startRumorMongering(extPacket *ExtendedGossipPacket) {
	peersWithRumor := []*net.UDPAddr{extPacket.SenderAddr}
	peers := gossiper.GetPeersAtomic()
	availablePeers := helpers.DifferenceString(peers, peersWithRumor)
	flipped := false

	if len(availablePeers) != 0 {
		randomPeer := gossiper.getRandomPeer(availablePeers)
		coin := 0
		for coin == 0 {
			statusReceived := gossiper.sendRumorWithTimeout(extPacket, randomPeer)
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
					if hw1 {
						fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
					}
					flipped = false
				}
			}
		}
	}
}

func (gossiper *Gossiper) sendRumorWithTimeout(extPacket *ExtendedGossipPacket, peer *net.UDPAddr) bool {

	rumorChan, _ := gossiper.getListenerForStatus(extPacket.Packet, peer.String())
	defer gossiper.deleteListenerForStatus(extPacket.Packet, peer.String())

	if hw1 {
		fmt.Println("MONGERING with " + peer.String())
	}

	gossiper.sendPacket(extPacket.Packet, peer)

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-rumorChan:
			if debug {
				fmt.Println("Got status")
			}
			return true

		case <-timer.C:
			if debug {
				fmt.Println("Timeout")
			}

			return false
		}
	}
}

func (gossiper *Gossiper) handlePeerStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		go gossiper.notifyListenersForStatus(extPacket)

		toSend := gossiper.getPeerStatusOtherNeeds(extPacket.Packet.Status.Want)

		if toSend != nil {
			packetToSend := gossiper.getPacketFromPeerStatus(*toSend)
			gossiper.sendPacket(packetToSend, extPacket.SenderAddr)
		} else {
			wanted := gossiper.doINeedSomething(extPacket.Packet.Status.Want)
			if wanted {
				statusToSend := gossiper.getStatusToSend()
				gossiper.sendPacket(statusToSend, extPacket.SenderAddr)
			} else {
				if hw1 {
					fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
				}
			}
		}
	}
}
