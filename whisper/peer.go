package whisper

//
//
//import (
//	"fmt"
//	mapset "github.com/deckarep/golang-set"
//	"github.com/dedis/protobuf"
//	"github.com/mikanikos/Peerster/gossiper"
//	"github.com/mikanikos/Peerster/helpers"
//	"math"
//	"net"
//	"sync"
//	"time"
//)
//
//// Peer represents a whisper protocol address connection.
//type Peer struct {
//	whisper *Whisper
//	address *net.UDPAddr
//
//	parameters sync.Map
//
//	known mapset.Set // Messages already known by the address to avoid wasting bandwidth
//
//	quit chan struct{}
//}
//
//// newPeer creates a new whisper address object, but does not run the handshake itself.
//func newPeer(whisper *Whisper, addr *net.UDPAddr) *Peer {
//	peer := &Peer{
//		whisper:        whisper,
//		address:        addr,
//		parameters: 	sync.Map{},
//		known:          mapset.NewSet(),
//		quit:           make(chan struct{}),
//	}
//
//	peer.parameters.Store(minPowIdx, 0.0)
//	peer.parameters.Store(bloomFilterIdx, GetEmptyBloomFilter())
//
//	return peer
//}
//
//// start initiates the address updater, periodically broadcasting the whisper packets
//// into the network.
//func (peer *Peer) start() {
//	go peer.doPeriodicUpdate()
//	fmt.Println("start", "address", peer.address.String())
//}
//
//// stop terminates the peer handler
//func (peer *Peer) stop() {
//	close(peer.quit)
//}
//
//// handshake sends the protocol initiation status message to the remote address and
//// verifies the remote status too.
//func (peer *Peer) handshake() error {
//	// Send the handshake status message asynchronously
//	//errc := make(chan error, 1)
//
//	pow := peer.whisper.GetMinPow()
//	bloom := peer.whisper.GetBloomFilter()
//
//	statusStruct := &Status{Bloom: bloom, Pow: pow}
//	packetToSend, err := protobuf.Encode(statusStruct)
//	if err != nil {
//		helpers.ErrorCheck(err, false)
//	}
//
//	wPacket := &gossiper.WhisperPacket{Code: statusCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: peer.whisper.gossiper.Name, ID: 0,}
//	gossipPacket := &gossiper.GossipPacket{WhisperPacket: wPacket}
//
//	fmt.Println("Sent to " + peer.address.String())
//	peer.whisper.gossiper.ConnectionHandler.sendPacket(gossipPacket, peer.address)
//
//	// start timer for stopping download
//	repeatTimer := time.NewTicker(time.Duration(5) * time.Second)
//	defer repeatTimer.Stop()
//
//	for {
//		select {
//		case packet := <-PeerChannels[peer.address.String()]:
//
//			fmt.Println("arrivatooo")
//
//			status := &Status{}
//			err = packet.DecodePacket(status)
//			if err != nil {
//				return fmt.Errorf("address [%x] sent bad status message: %v", peer.address.String(), err)
//			}
//
//			// subsequent parameters are optional
//			pow := status.Pow
//
//			if math.IsInf(pow, 0) || math.IsNaN(pow) || pow < 0.0 {
//				return fmt.Errorf("address [%x] sent bad status message: invalid pow", peer.address.String())
//			}
//
//			peer.parameters.Store(minPowIdx, pow)
//
//			bloom = status.Bloom
//
//			sz := len(bloom)
//			if sz != BloomFilterSize && sz != 0 {
//				return fmt.Errorf("address [%x] sent bad status message: wrong bloom filter size %d", peer.address.String(), sz)
//			}
//			peer.setBloomFilter(bloom)
//
//			return nil
//
//		case <-repeatTimer.C:
//			fmt.Println("Sent to " + peer.address.String())
//			peer.whisper.gossiper.ConnectionHandler.sendPacket(gossipPacket, peer.address)
//		}
//	}
//
//	// Fetch the remote status packet and verify protocol match
//
//	// extpacket := <-PacketChannels[address.address.String()]
//	// fmt.Println("maaaaaaaaaaa")
//
//	// if err := <-errc; err != nil {
//	// 	return fmt.Errorf("address [%x] failed to send status packet: %v", address.address.String(), err)
//	// }
//	return nil
//}
//
//// doPeriodicUpdate executes periodic operations on the address, including message transmission
//// and expiration.
//func (peer *Peer) doPeriodicUpdate() {
//	// Start the tickers for the updates
//	expire := time.NewTicker(expirationTimer)
//	defer expire.Stop()
//	transmit := time.NewTicker(broadcastTimer)
//	defer transmit.Stop()
//
//	// Loop and transmit until termination is requested
//	for {
//		select {
//		case <-expire.C:
//			peer.expire()
//
//		case <-transmit.C:
//			if err := peer.broadcast(); err != nil {
//				fmt.Println("broadcast failed", "reason", err, "address", peer.address.String())
//				return
//			}
//
//		case <-peer.quit:
//			return
//		}
//	}
//}
//
//// expire  removes all expired envelopes
//func (peer *Peer) expire() {
//	unmark := make(map[[32]byte]struct{})
//	peer.known.Each(func(v interface{}) bool {
//		if !peer.whisper.isEnvelopeCached(v.([32]byte)) {
//			unmark[v.([32]byte)] = struct{}{}
//		}
//		return true
//	})
//	// Dump all known but no longer cached
//	for hash := range unmark {
//		peer.known.Remove(hash)
//	}
//}
//
//// setKnown marks an envelope known
//func (peer *Peer) setKnown(envelope *Envelope) {
//	peer.known.Add(envelope.Hash())
//}
//
//// isKnown checks if an envelope is already known
//func (peer *Peer) isKnown(envelope *Envelope) bool {
//	return peer.known.Contains(envelope.Hash())
//}
//
//// broadcast iterates over the collection of envelopes and transmits yet unknown
//// ones over the network.
//func (peer *Peer) broadcast() error {
//	envelopes := peer.whisper.Envelopes()
//	bundle := make([]*Envelope, 0, len(envelopes))
//	for _, envelope := range envelopes {
//		pow, _ := peer.parameters.Load(minPowIdx)
//		bloom, _ := peer.parameters.Load(bloomFilterIdx)
//		if !peer.isKnown(envelope) && envelope.computePow() >= pow.(float64) && (CheckFilterMatch(bloom.([]byte), ConvertTopicToBloom(envelope.Topic))) {
//			bundle = append(bundle, envelope)
//		}
//	}
//
//	if len(bundle) > 0 {
//		// transmit the batch of envelopes
//
//		for _, env := range bundle {
//			peer.setKnown(env)
//			packetToSend, err := protobuf.Encode(env)
//			if err != nil {
//				return err
//			}
//			if err := peer.whisper.gossiper.SendWhisperPacket(messagesCode, packetToSend, peer.address); err != nil {
//				return err
//			}
//		}
//
//		fmt.Println("broadcast", "num. messages", len(bundle))
//	}
//	return nil
//}
//
//// ID returns a address's id
////func (address *Peer) ID() []byte {
////	id := address.address.ID()
////	return id[:]
////}
//
//func (peer *Peer) sendWhisperPacket(code uint32, data interface{}) error {
//
//	payload, err := protobuf.Encode(data)
//	if err != nil {
//		return err
//	}
//
//	wPacket := &gossiper.WhisperPacket{Code: code, Payload: payload, Size: uint32(len(payload)), Origin: peer.whisper.gossiper.Name, ID: 0,}
//	packet := &gossiper.GossipPacket{WhisperPacket: wPacket}
//	peer.whisper.gossiper.ConnectionHandler.sendPacket(packet, peer.address)
//
//	return nil
//}
//
//func (peer *Peer) setBloomFilter(bloom []byte) {
//	if !HasAnyFilter(bloom) || bloom == nil {
//		peer.parameters.Store(bloomFilterIdx, GetEmptyBloomFilter())
//	} else {
//		peer.parameters.Store(bloomFilterIdx, bloom)
//	}
//}
