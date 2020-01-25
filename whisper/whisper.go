package whisper

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/gossiper"
	"runtime"
	"sync"
	"time"
)

type SafeEnvelopes struct {
	Envelopes map[[32]byte]*Envelope
	Mutex     sync.RWMutex
}

type Whisper struct {
	// gossiper as underlying protocol
	gossiper *gossiper.Gossiper
	// main parameters (pow and bloom filter)
	parameters sync.Map
	// filters added for envelopes
	filters *FilterStorage
	// crypto keys (both private and symmetric) storage with unique id
	cryptoKeys sync.Map
	// envelopes which are not expired yet
	envelopes *SafeEnvelopes
	// routing envelopes according to received bloom filters
	routingHandler *RoutingHandler

	messageQueue chan *Envelope // ReceivedMessage queue for normal whisper messages
	quit         chan struct{}  // Channel used for graceful exit
}

// NewWhisper creates new Whisper instance
func NewWhisper(g *gossiper.Gossiper) *Whisper {

	whisper := &Whisper{
		gossiper:       g,
		parameters:     sync.Map{},
		cryptoKeys:     sync.Map{},
		envelopes:      &SafeEnvelopes{Envelopes: make(map[[32]byte]*Envelope)},
		filters:        NewFilterStorage(),
		routingHandler: NewRoutingHandler(),
		messageQueue:   make(chan *Envelope, messageQueueLimit),
		quit:           make(chan struct{}),
	}

	whisper.parameters.Store(minPowIdx, DefaultMinimumPoW)
	whisper.parameters.Store(maxMsgSizeIdx, DefaultMaxMessageSize)
	whisper.parameters.Store(bloomFilterIdx, GetEmptyBloomFilter())

	return whisper
}

// process tlc message
func (whisper *Whisper) processWhisperStatus() {
	for extPacket := range gossiper.PacketChannels["whisperStatus"] {
		go whisper.gossiper.HandleGossipMessage(extPacket, extPacket.Packet.WhisperStatus.Origin, extPacket.Packet.WhisperStatus.ID)
		whisper.routingHandler.updateRoutingTable(extPacket.Packet.WhisperStatus, extPacket.SenderAddr)
	}
}

// process tlc message
func (whisper *Whisper) processWhisperPacket() {
	for extPacket := range gossiper.PacketChannels["whisperPacket"] {

		packet := extPacket.Packet.WhisperPacket

		if packet.Code == messagesCode {
			// decode the contained envelope

			fmt.Println("Got envelope")

			envelope := &Envelope{}

			err := protobuf.Decode(packet.Payload, envelope)
			if err != nil {
				fmt.Println(err)
			}

			whisper.handleEnvelope(envelope)
			if err != nil {
				fmt.Println("bad envelope received, address will be disconnected")
				//disconnectPeer
			}
		}
	}
}

// GetEnvelope retrieves an envelope from its hash
func (whisper *Whisper) GetEnvelope(hash [32]byte) *Envelope {
	whisper.envelopes.Mutex.RLock()
	defer whisper.envelopes.Mutex.RUnlock()
	return whisper.envelopes.Envelopes[hash]
}

// GetMinPow returns the Pow value required
func (whisper *Whisper) GetMinPow() float64 {
	val, loaded := whisper.parameters.Load(minPowIdx)
	if !loaded {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

func (whisper *Whisper) GetMinPowTolerated() float64 {
	val, exist := whisper.parameters.Load(minPowToleranceIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

// GetBloomFilter returns the aggregated bloom filter for all the topics of interest
func (whisper *Whisper) GetBloomFilter() []byte {
	value, loaded := whisper.parameters.Load(bloomFilterIdx)
	if !loaded {
		return nil
	}
	return value.([]byte)
}

// GetBloomFilterTolerated returns the bloom filter tolerated for a limited time
func (whisper *Whisper) GetBloomFilterTolerated() []byte {
	val, exist := whisper.parameters.Load(bloomFilterToleranceIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

// MaxMessageSize returns the maximum accepted message size.
//func (whisper *Whisper) MaxMessageSize() uint32 {
//	val, _ := whisper.settings.Load(maxMsgSizeIdx)
//	return val.(uint32)
//}

// SetMaxMessageSize sets the maximal message size allowed by this node
//func (whisper *Whisper) SetMaxMessageSize(size uint32) error {
//	if size > MaxMessageSize {
//		return fmt.Errorf("message size too large [%d>%d]", size, MaxMessageSize)
//	}
//	whisper.settings.Store(maxMsgSizeIdx, size)
//	return nil
//}

// SetBloomFilter sets the new bloom filter
func (whisper *Whisper) SetBloomFilter(bloom []byte) error {
	if len(bloom) != BloomFilterSize {
		return fmt.Errorf("invalid bloom filter size")
	}

	whisper.parameters.Store(bloomFilterIdx, bloom)

	// broadcast the update
	wPacket := &gossiper.WhisperStatus{Code: bloomFilterExCode, Bloom: bloom}
	whisper.gossiper.SendWhisperStatus(wPacket)

	go func() {
		// allow some time before all the peers have processed the notification
		time.Sleep(time.Duration(DefaultSyncAllowance) * time.Second)
		whisper.parameters.Store(bloomFilterToleranceIdx, bloom)
	}()

	return nil
}

// SetMinPoW sets the minimal PoW required by this node
func (whisper *Whisper) SetMinPoW(pow float64) error {
	if pow < 0.0 {
		return fmt.Errorf("invalid pow")
	}

	whisper.parameters.Store(minPowIdx, pow)

	// broadcast the update
	wPacket := &gossiper.WhisperStatus{Code: powRequirementCode, Pow: pow}
	whisper.gossiper.SendWhisperStatus(wPacket)

	go func() {
		// allow some time before all the peers have processed the notification
		time.Sleep(time.Duration(DefaultSyncAllowance) * time.Second)
		whisper.parameters.Store(minPowToleranceIdx, pow)
	}()

	return nil
}

//func (whisper *Whisper) notifyPeersAboutBloomFilterChange(bloom []byte) {
//	arr := whisper.getPeers()
//	for _, p := range arr {
//		err := p.notifyAboutBloomFilterChange(bloom)
//		if err != nil {
//			// allow one retry
//			err = p.notifyAboutBloomFilterChange(bloom)
//		}
//		if err != nil {
//			fmt.Println("failed to notify address about new bloom filter", "address", p.address.String(), "error", err)
//		}
//	}
//}

// Subscribe installs a new message handler used for filtering, decrypting
// and subsequent storing of incoming messages.
func (whisper *Whisper) Subscribe(f *Filter) (string, error) {
	s, err := whisper.filters.AddFilter(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}
	return s, err
}

// updateBloomFilter recomputes bloom filter,
func (whisper *Whisper) updateBloomFilter(f *Filter) {
	aggregate := make([]byte, BloomFilterSize)
	for _, t := range f.Topics {
		top := ConvertBytesToTopic(t)
		b := ConvertTopicToBloom(top)
		aggregate = AggregateBloom(aggregate, b)
	}

	if !CheckFilterMatch(whisper.GetBloomFilter(), aggregate) {
		aggregate = AggregateBloom(whisper.GetBloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

// GetFilterFromID returns the filter by id.
func (whisper *Whisper) GetFilter(id string) *Filter {
	return whisper.filters.GetFilterFromID(id)
}

// Unsubscribe removes an installed message handler.
//func (whisper *Whisper) Unsubscribe(id string) error {
//	ok := whisper.filters.RemoveFilter(id)
//	if !ok {
//		return fmt.Errorf("Unsubscribe: Invalid ID")
//	}
//	return nil
//}

// Send injects a message into the whisper send queue, to be distributed in the
// network in the coming cycles.
func (whisper *Whisper) Send(envelope *Envelope) error {
	err := whisper.handleEnvelope(envelope)
	if err != nil {
		return fmt.Errorf("failed to handle Envelope envelope")
	}
	return err
}

// Start implements node.Service, starting the background data propagation thread
// of the Whisper protocol.
func (whisper *Whisper) Start() error {
	go whisper.processWhisperPacket()
	go whisper.processWhisperStatus()
	go whisper.update()
	go whisper.sendStatusPeriodically()

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go whisper.processQueue()
	}

	return nil
}

// Stop implements node.Service, stopping the background data propagation thread
// of the Whisper protocol.
func (whisper *Whisper) Stop() error {
	close(whisper.quit)
	fmt.Println("whisper stopped")
	return nil
}

//func (whisper *Whisper) BroadcastStatusPacket() {
//	pow := whisper.GetMinPow()
//	bloom := whisper.GetBloomFilter()
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
//	peer.whisper.gossiper.ConnectionHandler.SendPacket(gossipPacket, peer.address)
//}

//func (whisper *Whisper) SendStatusPacket(peer *Peer) {
//	pow := whisper.GetMinPow()
//	bloom := whisper.GetBloomFilter()
//
//	statusStruct := &Status{Bloom: bloom, Pow: pow}
//	packetToSend, err := protobuf.Encode(statusStruct)
//	if err != nil {
//		helpers.ErrorCheck(err, false)
//	}
//
//	wPacket := &gossiper.WhisperPacket{Code: statusCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: whisper.gossiper.Name, ID: 0,}
//	gossipPacket := &gossiper.GossipPacket{WhisperPacket: wPacket}
//
//	whisper.gossiper.ConnectionHandler.sendPacket(gossipPacket, peer.address)
//}

////HandlePeer is called by the underlying P2P layer when the whisper sub-protocol
////connection is negotiated.
//func (whisper *Whisper) HandlePeer(peer *net.UDPAddr) error {
//
//	fmt.Println("Handle " + peer.String())
//
//	whisperPeer, err := whisper.getPeer(peer.String())
//	if err != nil {
//		fmt.Println("No such address")
//		return err
//	}
//
//	defer func() {
//		whisper.peerMu.Lock()
//		delete(whisper.peers, whisperPeer)
//		whisper.peerMu.Unlock()
//	}()
//
//	// Run the address handshake and state updates
//	if err := whisperPeer.handshake(); err != nil {
//		fmt.Println("Handshake failed")
//		return err
//	} else {
//		fmt.Println("Handshake ok")
//	}
//	whisperPeer.start()
//	defer whisperPeer.stop()
//
//	loop := whisper.runMessageLoop(whisperPeer)
//
//	fmt.Println("Error")
//
//	fmt.Println(loop)
//
//	return loop
//}
//
//// runMessageLoop reads and processes inbound messages directly to merge into client-global state.
//func (whisper *Whisper) runMessageLoop(p *Peer) error {
//	for packet := range PeerChannels[p.address.String()] {
//
//		//// fetch the next packet
//		//packet, err := rw.ReadMsg()
//		//if err != nil {
//		//	fmt.Println("message loop", "address", p.address.ID(), "err", err)
//		//	return err
//		//}
//		if packet.Size > DefaultMaxMessageSize {
//			//fmt.Println("oversized message received", "address", p.address.String())
//			return fmt.Errorf("oversized message received")
//		}
//
//		switch packet.Code {
//		case statusCode:
//			pow := p.whisper.GetMinPow()
//			bloom := p.whisper.GetBloomFilter()
//
//			status := &Status{Bloom: bloom, Pow: pow}
//			if err := p.sendWhisperPacket(statusCode, status); err != nil {
//				fmt.Println("Failed sending status to address")
//			}
//			//packetToSend, err := protobuf.Encode(statusStruct)
//			//helpers.ErrorCheck(err, false)
//			//
//			//wPacket := &gossiper.WhisperPacket{Code: statusCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: whisper.gossiper.Name, ID: 0,}
//			//gossipPacket := &gossiper.GossipPacket{WhisperPacket: wPacket}
//			//
//			//fmt.Println("Sent to " + p.address.String())
//			//whisper.gossiper.ConnectionHandler.sendPacket(gossipPacket, p.address)
//
//		case messagesCode:
//			// decode the contained envelope
//
//			fmt.Println("Got envelope")
//
//			envelope := &Envelope{}
//			if err := packet.DecodePacket(envelope); err != nil {
//				fmt.Println("failed to decode envelopes, address will be disconnected", "address", p.address.String(), "err", err)
//				return fmt.Errorf("invalid envelopes")
//			}
//
//			trouble := false
//			cached, err := whisper.add(envelope)
//			if err != nil {
//				trouble = true
//				fmt.Println("bad envelope received, address will be disconnected", "address", p.address.String(), "err", err)
//			}
//			if cached {
//				p.setKnown(envelope)
//			}
//
//			if trouble {
//				return fmt.Errorf("invalid envelope")
//			}
//
//			//trouble := false
//			//for _, env := range envelopes {
//			//	cached, err := whisper.handleEnvelope(env, whisper.LightClientMode())
//			//	if err != nil {
//			//		trouble = true
//			//		fmt.Println("bad envelope received, address will be disconnected", "address", p.address.ID(), "err", err)
//			//	}
//			//	if cached {
//			//		p.setKnown(env)
//			//	}
//			//}
//
//			//if trouble {
//			//	return fmt.Errorf("invalid envelope")
//			//}
//		case powRequirementCode:
//			var i uint64
//			err := packet.DecodePacket(&i)
//			if err != nil {
//				fmt.Println("failed to decode powRequirementCode message, address will be disconnected", "address", p.address.String(), "err", err)
//				return fmt.Errorf("invalid powRequirementCode message")
//			}
//			f := math.Float64frombits(i)
//			if math.IsInf(f, 0) || math.IsNaN(f) || f < 0.0 {
//				fmt.Println("invalid value in powRequirementCode message, address will be disconnected", "address", p.address.String(), "err", err)
//				return fmt.Errorf("invalid value in powRequirementCode message")
//			}
//			p.parameters.Store(minPowIdx, f)
//
//		case bloomFilterExCode:
//			var bloom []byte
//			err := packet.DecodePacket(&bloom)
//			if err == nil && len(bloom) != BloomFilterSize {
//				err = fmt.Errorf("wrong bloom filter size %d", len(bloom))
//			}
//
//			if err != nil {
//				fmt.Println("failed to decode bloom filter exchange message, address will be disconnected", "address", p.address.String(), "err", err)
//				return fmt.Errorf("invalid bloom filter exchange message")
//			}
//			p.setBloomFilter(bloom)
//		default:
//			// NewWhisper message types might be implemented in the future versions of Whisper.
//			// For forward compatibility, just ignore.
//		}
//
//		//packet.Discard()
//	}
//
//	fmt.Println("aoooooooooooooo")
//
//	return nil
//}

// handleEnvelope inserts a new envelope into the message pool to be distributed within the
// whisper network. It also inserts the envelope into the expiration pool at the
// appropriate time-stamp. In case of error, connection should be dropped.
func (whisper *Whisper) handleEnvelope(envelope *Envelope) error {
	now := uint32(time.Now().Unix())

	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		if sent-DefaultSyncAllowance > now {
			return fmt.Errorf("envelope created in the future [%x]", envelope.Hash())
		}
		envelope.computePow(sent - now + 1)
	}

	if envelope.Expiry < now {
		if envelope.Expiry+DefaultSyncAllowance*2 < now {
			return fmt.Errorf("very old message")
		}
		fmt.Println("expired envelope dropped")
		return nil
	}

	if uint32(envelope.size()) > MaxMessageSize {
		return fmt.Errorf("huge messages are not allowed")
	}

	if envelope.GetPow() < whisper.GetMinPow() {
		if envelope.GetPow() < whisper.GetMinPowTolerated() {
			return fmt.Errorf("envelope with low pow received")
		}
	}

	if !CheckFilterMatch(whisper.GetBloomFilter(), envelope.GetBloom()) {
		if !CheckFilterMatch(whisper.GetBloomFilterTolerated(), envelope.GetBloom()) {
			return fmt.Errorf("envelope does not match bloom filter")
		}
	}

	hash := envelope.Hash()

	whisper.envelopes.Mutex.Lock()

	_, loaded := whisper.envelopes.Envelopes[hash]
	if !loaded {
		whisper.envelopes.Envelopes[hash] = envelope
		go func(e *Envelope) {
			for len(whisper.messageQueue) >= messageQueueLimit {
				time.Sleep(time.Second)
			}
			whisper.messageQueue <- envelope
		}(envelope)
	} else {
		fmt.Println("whisper envelope already present")
	}

	whisper.envelopes.Mutex.Unlock()

	return nil
}

// postEvent queues the message for further processing.
//func (whisper *Whisper) postEvent(envelope *Envelope) {
//	//whisper.checkOverflow()
//	whisper.messageQueue <- envelope
//
//}

// checkOverflow checks if message queue overflow occurs and reports it if necessary.
//func (whisper *Whisper) checkOverflow() {
//	queueSize := len(whisper.messageQueue)
//
//	if queueSize == messageQueueLimit {
//		if !whisper.Overflow() {
//			whisper.settings.Store(overflowIdx, true)
//			fmt.Println("message queue overflow")
//		}
//	} else if queueSize <= messageQueueLimit/2 {
//		if whisper.Overflow() {
//			whisper.settings.Store(overflowIdx, false)
//			fmt.Println("message queue overflow fixed (back to normal)")
//		}
//	}
//}

// processQueue delivers the messages to the subscribers during the lifetime of the whisper node.
func (whisper *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-whisper.quit:
			return

		case e = <-whisper.messageQueue:
			whisper.forwardEnvelope(e)
			whisper.filters.NotifySubscribers(e)
		}
	}
}

// doPeriodicUpdate loops until the lifetime of the whisper node, updating its internal
// state by expiring stale messages from the pool.
func (whisper *Whisper) update() {
	// Start a ticker to check for expirations
	expire := time.NewTicker(expirationTimer)

	// Repeat updates until termination is requested
	for {
		select {
		case <-expire.C:
			whisper.expire()

		case <-whisper.quit:
			return
		}
	}
}

// expire iterates over all the expiration timestamps, removing all stale
// messages from the pools.
func (whisper *Whisper) expire() {
	whisper.envelopes.Mutex.Lock()
	defer whisper.envelopes.Mutex.Unlock()

	now := uint32(time.Now().Unix())
	for hash, env := range whisper.envelopes.Envelopes {
		if env.Expiry < now {
			delete(whisper.envelopes.Envelopes, hash)
		}
	}
}

// Envelopes retrieves all the envelopes
func (whisper *Whisper) Envelopes() []*Envelope {
	whisper.envelopes.Mutex.RLock()
	defer whisper.envelopes.Mutex.RUnlock()

	array := make([]*Envelope, 0, len(whisper.envelopes.Envelopes))
	for _, envelope := range whisper.envelopes.Envelopes {
		array = append(array, envelope)
	}
	return array
}

//// isEnvelopePresent checks if envelope with specific hash has already been cached
//func (whisper *Whisper) isEnvelopePresent(hash [32]byte) bool {
//	whisper.envelopes.Mutex.Lock()
//	defer whisper.envelopes.Mutex.Unlock()
//
//	_, exist := whisper.envelopes.Envelopes[hash]
//	return exist
//}
