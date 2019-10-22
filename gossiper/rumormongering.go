package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
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

	// if gossiper.isMongering[peer.String()] {
	// 	return false
	// }

	// gossiper.isMongering[peer.String()] = true

	//gossiper.creatingRumorSyncChannels(peer.String())

	rumorChan, _ := gossiper.mongeringChannels.LoadOrStore(peer.String(), make(chan bool, 0))

	go gossiper.sendPacket(packet, peer)
	fmt.Println("MONGERING with " + peer.String())

	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)

	fmt.Println("Waiting for status")

	for {
		select {
		case <-rumorChan.(chan bool):
			timer.Stop()
			fmt.Println("Got status")
			//gossiper.isMongering[peer.String()] = false
			return true

		case <-timer.C:
			fmt.Println("Timeout")
			//gossiper.isMongering[peer.String()] = false
			return false
		}
	}
}

func (gossiper *Gossiper) createStatus() []PeerStatus {
	//gossiper.originPackets.Mutex.Lock()
	//defer gossiper.originPackets.Mutex.Unlock()
	myStatus := make([]PeerStatus, 0)
	// for origin, idMessages := range gossiper.originPackets.OriginPacketsMap {
	// 	var maxID uint32 = 0
	// 	for id := range idMessages {
	// 		if id > maxID {
	// 			maxID = id
	// 		}
	// 	}
	// 	myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: maxID + 1})
	// }

	gossiper.originPackets.OriginPacketsMap.Range(func(key interface{}, value interface{}) bool {
		origin := key.(string)
		idMessages := value.(*sync.Map)
		var maxID uint32 = 0

		idMessages.Range(func(key interface{}, value interface{}) bool {
			id := key.(uint32)
			if id > maxID {
				maxID = id
			}
			return true
		})

		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: maxID + 1})
		return true
	})

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
	// gossiper.originPackets.Mutex.Lock()
	// defer gossiper.originPackets.Mutex.Unlock()
	// idMessages, _ := gossiper.originPackets.OriginPacketsMap[ps.Identifier]
	// maxID := ps.NextID
	// for id := range idMessages {
	// 	if id > maxID {
	// 		maxID = id
	// 	}
	// }
	// packets := make([]*GossipPacket, 0)
	// for i := ps.NextID; i <= maxID; i++ {
	// 	packets = append(packets, idMessages[i].Packet)
	// }

	value, _ := gossiper.originPackets.OriginPacketsMap.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	maxID := ps.NextID
	idMessages.Range(func(key interface{}, value interface{}) bool {
		id := key.(uint32)
		if id > maxID {
			maxID = id
		}
		return true
	})

	packets := make([]*GossipPacket, 0)
	for i := ps.NextID; i <= maxID; i++ {
		valuePacket, _ := idMessages.Load(i)
		packet := valuePacket.(*GossipPacket)
		packets = append(packets, packet)
	}

	return packets
}

func (gossiper *Gossiper) handlePeerStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		myStatus := gossiper.createStatus()
		toSend := gossiper.getDifferenceStatus(myStatus, extPacket.Packet.Status.Want)

		if len(toSend) != 0 {
			gossiper.sendPacketFromStatus(toSend, extPacket.SenderAddr)
		} else {
			wanted := gossiper.getDifferenceStatus(extPacket.Packet.Status.Want, myStatus)
			if len(wanted) != 0 {
				gossiper.sendStatusPacket(extPacket.SenderAddr)
			} else {
				fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
				//gossiper.createOrGetSyncCond(extPacket.SenderAddr.String()).Broadcast()
				// if gossiper.isMongering[extPacket.SenderAddr.String()] {
				//go gossiper.closeSyncChannel(extPacket.SenderAddr.String())
				// }
			}
		}
	}
}
