package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) startRumorMongering(extPacket *ExtendedGossipPacket, origin string, id uint32) {
	peersWithRumor := []*net.UDPAddr{extPacket.SenderAddr}
	peers := gossiper.GetPeersAtomic()
	availablePeers := helpers.DifferenceString(peers, peersWithRumor)
	flipped := false

	if len(availablePeers) != 0 {
		randomPeer := getRandomPeer(availablePeers)
		coin := 0
		for coin == 0 {
			statusReceived := gossiper.sendRumorWithTimeout(extPacket, origin, id, randomPeer)
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
				randomPeer = getRandomPeer(availablePeers)

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

func (gossiper *Gossiper) sendRumorWithTimeout(extPacket *ExtendedGossipPacket, origin string, id uint32, peer *net.UDPAddr) bool {

	rumorChan, _ := gossiper.getListenerForStatus(origin, id, peer.String())
	defer gossiper.deleteListenerForStatus(origin, id, peer.String())

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

		toSend := getPeerStatusForPeer(extPacket.Packet.Status.Want, &gossiper.myStatus)
		if toSend != nil {
			packetToSend := gossiper.getPacketFromPeerStatus(*toSend)
			gossiper.sendPacket(packetToSend, extPacket.SenderAddr)
		} else {
			wanted := isPeerStatusNeeded(extPacket.Packet.Status.Want, &gossiper.myStatus)
			if wanted {
				statusToSend := getStatusToSend(&gossiper.myStatus)
				gossiper.sendPacket(&GossipPacket{Status: statusToSend}, extPacket.SenderAddr)
			} else {
				if hw1 {
					fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
				}
			}
		}
	}
}
