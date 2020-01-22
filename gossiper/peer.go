// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package gossiper

import (
	"fmt"
	mapset "github.com/deckarep/golang-set"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/helpers"
	"math"
	"net"
	"sync"
	"time"
)

// Peer represents a whisper protocol peer connection.
type Peer struct {
	host *Whisper
	peer *net.UDPAddr

	trusted        bool
	powRequirement float64
	bloomMu        sync.Mutex
	bloomFilter    []byte
	fullNode       bool

	known mapset.Set // Messages already known by the peer to avoid wasting bandwidth

	quit chan struct{}
}

// newPeer creates a new whisper peer object, but does not run the handshake itself.
func newPeer(host *Whisper, remote *net.UDPAddr) *Peer {
	return &Peer{
		host:           host,
		peer:           remote,
		trusted:        false,
		powRequirement: 0.0,
		known:          mapset.NewSet(),
		quit:           make(chan struct{}),
		bloomFilter:    MakeFullNodeBloom(),
		fullNode:       true,
	}
}

// start initiates the peer updater, periodically broadcasting the whisper packets
// into the network.
func (peer *Peer) start() {
	go peer.update()
	fmt.Println("start", "peer", peer.peer.String())
}

// stop terminates the peer updater, stopping message forwarding to it.
func (peer *Peer) stop() {
	close(peer.quit)
	fmt.Println("stop", "peer", peer.peer.String())
}

type WhisperStatus struct {
	Pow   uint64
	Bloom []byte
}

// handshake sends the protocol initiation status message to the remote peer and
// verifies the remote status too.
func (peer *Peer) handshake() error {
	// Send the handshake status message asynchronously
	//errc := make(chan error, 1)

	go func() {
		pow := peer.host.MinPow()
		powConverted := math.Float64bits(pow)
		bloom := peer.host.BloomFilter()

		status := &WhisperStatus{Bloom: bloom, Pow: powConverted}
		packetToSend, err := protobuf.Encode(status)
		if err != nil {
			helpers.ErrorCheck(err, false)
		}

		wPacket := &WhisperPacket{Code: statusCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: peer.host.gossiper.name, ID: 0,}
		packet := &GossipPacket{WhisperPacket: wPacket}
		peer.host.gossiper.connectionHandler.sendPacket(packet, peer.peer)

		fmt.Println("Sent to " + peer.peer.String())
	}()

	fmt.Println("Waiting " + peer.peer.String())

	extPacket := <-PeerChannels[peer.peer.String()]
	
	fmt.Println("arrivatooo")

	packet := extPacket.Packet.WhisperPacket
	status := &WhisperStatus{}
	err := packet.DecodeStatus(status)
	if err != nil {
		return fmt.Errorf("peer [%x] sent bad status message: %v", peer.peer.String(), err)
	}

	// subsequent parameters are optional
	powRaw := status.Pow

	pow := math.Float64frombits(powRaw)
	if math.IsInf(pow, 0) || math.IsNaN(pow) || pow < 0.0 {
		return fmt.Errorf("peer [%x] sent bad status message: invalid pow", peer.peer.String())
	}
	peer.powRequirement = pow

	bloom := status.Bloom

	sz := len(bloom)
	if sz != BloomFilterSize && sz != 0 {
		return fmt.Errorf("peer [%x] sent bad status message: wrong bloom filter size %d", peer.peer.String(), sz)
	}
	peer.setBloomFilter(bloom)

	// Fetch the remote status packet and verify protocol match

	// extpacket := <-PacketChannels[peer.peer.String()]
	// fmt.Println("maaaaaaaaaaa")
	
	// if err := <-errc; err != nil {
	// 	return fmt.Errorf("peer [%x] failed to send status packet: %v", peer.peer.String(), err)
	// }
	return nil
}

// update executes periodic operations on the peer, including message transmission
// and expiration.
func (peer *Peer) update() {
	// Start the tickers for the updates
	expire := time.NewTicker(expirationCycle)
	defer expire.Stop()
	transmit := time.NewTicker(transmissionCycle)
	defer transmit.Stop()

	// Loop and transmit until termination is requested
	for {
		select {
		case <-expire.C:
			peer.expire()

		case <-transmit.C:
			if err := peer.broadcast(); err != nil {
				fmt.Println("broadcast failed", "reason", err, "peer", peer.peer.String())
				return
			}

		case <-peer.quit:
			return
		}
	}
}

// expire iterates over all the known envelopes in the host and removes all
// expired (unknown) ones from the known list.
func (peer *Peer) expire() {
	unmark := make(map[[32]byte]struct{})
	peer.known.Each(func(v interface{}) bool {
		if !peer.host.isEnvelopeCached(v.([32]byte)) {
			unmark[v.([32]byte)] = struct{}{}
		}
		return true
	})
	// Dump all known but no longer cached
	for hash := range unmark {
		peer.known.Remove(hash)
	}
}

// broadcast iterates over the collection of envelopes and transmits yet unknown
// ones over the network.
func (peer *Peer) broadcast() error {
	envelopes := peer.host.Envelopes()
	bundle := make([]*Envelope, 0, len(envelopes))
	for _, envelope := range envelopes {
		if envelope.PoW() >= peer.powRequirement && peer.bloomMatch(envelope) {
			bundle = append(bundle, envelope)
		}
	}

	if len(bundle) > 0 {
		// transmit the batch of envelopes

		for _, env := range bundle {
			packetToSend, err := protobuf.Encode(&env)
			if err != nil {
				return err
			}
			if err := peer.host.gossiper.SendWhisperPacket(messagesCode, packetToSend); err != nil {
				return err
			}
		}

		fmt.Println("broadcast", "num. messages", len(bundle))
	}
	return nil
}

// ID returns a peer's id
//func (peer *Peer) ID() []byte {
//	id := peer.peer.ID()
//	return id[:]
//}

func (peer *Peer) notifyAboutPowRequirementChange(pow float64) error {
	i := math.Float64bits(pow)
	packetToSend, err := protobuf.Encode(&i)
	if err != nil {
		return err
	} 

	wPacket := &WhisperPacket{Code: powRequirementCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: peer.host.gossiper.name, ID: 0,}
	packet := &GossipPacket{WhisperPacket: wPacket}
	peer.host.gossiper.connectionHandler.sendPacket(packet, peer.peer)

	return nil
}

func (peer *Peer) notifyAboutBloomFilterChange(bloom []byte) error {
	packetToSend, err := protobuf.Encode(&bloom)
	if err != nil {
		return err
	}
	wPacket := &WhisperPacket{Code: bloomFilterExCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: peer.host.gossiper.name, ID: 0,}
	packet := &GossipPacket{WhisperPacket: wPacket}
	peer.host.gossiper.connectionHandler.sendPacket(packet, peer.peer)

	return nil
}

func (peer *Peer) bloomMatch(env *Envelope) bool {
	peer.bloomMu.Lock()
	defer peer.bloomMu.Unlock()
	return peer.fullNode || BloomFilterMatch(peer.bloomFilter, env.Bloom())
}

func (peer *Peer) setBloomFilter(bloom []byte) {
	peer.bloomMu.Lock()
	defer peer.bloomMu.Unlock()
	peer.bloomFilter = bloom
	peer.fullNode = isFullNode(bloom)
	if peer.fullNode && peer.bloomFilter == nil {
		peer.bloomFilter = MakeFullNodeBloom()
	}
}

func MakeFullNodeBloom() []byte {
	bloom := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		bloom[i] = 0xFF
	}
	return bloom
}
