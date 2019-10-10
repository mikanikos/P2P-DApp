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
	coin := 1

	peersWithRumor := make([]*net.UDPAddr, 0)
	peersWithRumor = append(peersWithRumor, extPacket.SenderAddr)

	for coin == 1 {
		peers := gossiper.GetPeersAtomic()
		availablePeers := helpers.DifferenceString(peers, peersWithRumor)
		if len(availablePeers) == 0 {
			return
		}
		indexPeer := rand.Intn(len(availablePeers))
		randomPeer := availablePeers[indexPeer]
		peersWithRumor = append(peersWithRumor, randomPeer)

		fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
		statusReceived := gossiper.sendRumorWithTimeout(extPacket.Packet, randomPeer)

		if statusReceived {
			coin = rand.Int() % 2
		}
	}
}

func (gossiper *Gossiper) sendRumorWithTimeout(packet *GossipPacket, peer *net.UDPAddr) bool {
	gossiper.statusChannels[peer.String()] = make(chan *ExtendedGossipPacket)

	gossiper.sendPacket(packet, peer)
	fmt.Println("MONGERING with " + peer.String())

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-gossiper.statusChannels[peer.String()]:
			return true
		case <-timer.C:
			return false
		}
	}
}

func (gossiper *Gossiper) createStatus() []PeerStatus {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	myStatus := make([]PeerStatus, 0)
	for origin, idMessages := range gossiper.originPackets.OriginPacketsMap {
		var maxID uint32 = 0
		for id := range idMessages {
			if id > maxID {
				maxID = id
			}
		}
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: maxID + 1})
	}
	return myStatus
}

func (gossiper *Gossiper) getDifferenceStatus(myStatus, otherStatus []PeerStatus) []PeerStatus {
	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}
	difference := make([]PeerStatus, 0)
	for _, elem := range myStatus {
		id, isOriginKnown := originIDMap[elem.Identifier]
		if !isOriginKnown {
			difference = append(difference, PeerStatus{Identifier: elem.Identifier, NextID: 1})
		} else if elem.NextID > id {
			difference = append(difference, PeerStatus{Identifier: elem.Identifier, NextID: id})
		}
	}
	return difference
}

func (gossiper *Gossiper) getPacketsFromStatus(ps PeerStatus) []*GossipPacket {
	gossiper.originPackets.Mutex.Lock()
	defer gossiper.originPackets.Mutex.Unlock()
	idMessages, _ := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
	maxID := ps.NextID
	for id := range idMessages {
		if id > maxID {
			maxID = id
		}
	}
	packets := make([]*GossipPacket, 0)
	for i := ps.NextID; i <= maxID; i++ {
		packets = append(packets, idMessages[i].Packet)
	}

	return packets
}
