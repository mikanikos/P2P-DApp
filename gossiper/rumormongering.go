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

func (gossiper *Gossiper) startRumorMongering(extPacket *ExtendedGossipPacket, flipNow bool) {
	peersWithRumor := []*net.UDPAddr{extPacket.SenderAddr}
	peers := gossiper.GetPeersAtomic()
	availablePeers := helpers.DifferenceString(peers, peersWithRumor)

	if len(availablePeers) != 0 {
		randomPeer := gossiper.getRandomPeer(availablePeers)

		coin := 0
		if flipNow {
			coin = rand.Int() % 2
		}

		for coin == 0 {

			if flipNow {
				fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
				flipNow = false
			}

			statusReceived := gossiper.sendRumorWithTimeout(extPacket, randomPeer)
			if statusReceived {
				return
				//coin = rand.Int() % 2
				//flipped = true
			}

			delete(gossiper.currentMonger, randomPeer.String())

			peers := gossiper.GetPeersAtomic()
			peersWithRumor = []*net.UDPAddr{randomPeer}
			availablePeers := helpers.DifferenceString(peers, peersWithRumor)
			if len(availablePeers) == 0 {
				return
			}
			randomPeer = gossiper.getRandomPeer(availablePeers)

		}
	}
}

func (gossiper *Gossiper) sendRumorWithTimeout(extPacket *ExtendedGossipPacket, peer *net.UDPAddr) bool {

	rumorChan, isMongering := gossiper.getListenerForStatus(extPacket.Packet, peer.String())

	if isMongering {
		fmt.Println("Already mongering")
		return false
	}

	gossiper.currentMonger[peer.String()] = extPacket
	defer gossiper.deleteListenerForStatus(extPacket.Packet, peer.String())

	fmt.Println("MONGERING with " + peer.String())
	gossiper.sendPacket(extPacket.Packet, peer)

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-rumorChan:
			return true

		case <-timer.C:
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
				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
				gossiper.notifySync(extPacket.SenderAddr.String())
			}
		}
	}
}

func (gossiper *Gossiper) notifySync(peer string) {
	if packet, exists := gossiper.currentMonger[peer]; exists {

		delete(gossiper.currentMonger, peer)

		go gossiper.startRumorMongering(packet, true)
	}
}
