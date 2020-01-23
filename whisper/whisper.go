package whisper

import (
	"encoding/hex"
	"fmt"
	"github.com/mikanikos/Peerster/gossiper"
	"math"
	"net"
	"runtime"
	"sync"
	"time"
)

var PeerChannels map[string]chan *gossiper.WhisperPacket

type Status struct {
	Pow   float64
	Bloom []byte
}

type SafeEnvelopes struct {
	Envelopes   map[[32]byte]*Envelope
	Mutex 		sync.RWMutex
}

type Whisper struct {
	// gossiper as underlying protocol
	gossiper *gossiper.Gossiper
	// main parameters (pow and bloom filter)
	parameters sync.Map
	// filters added for envelopes
	filters  *Filters
	// crypto keys (both private and symmetric) storage with unique id
	cryptoKeys sync.Map
	// envelopes which are not expired yet
	envelopes *SafeEnvelopes

	peerMu sync.RWMutex       // Mutex to sync the active peer set
	peers  map[*Peer]struct{} // Set of currently active peers

	messageQueue chan *Envelope // Message queue for normal whisper messages
	//p2pMsgQueue  chan *Envelope // Message queue for peer-to-peer messages (not to be forwarded any further)
	quit         chan struct{}  // Channel used for graceful exit
}

// New creates a Whisper client ready to communicate through the Ethereum P2P network.
func New(g *gossiper.Gossiper) *Whisper {

	whisper := &Whisper{
		gossiper: g,
		parameters: sync.Map{},
		cryptoKeys: sync.Map{},
		envelopes:     &SafeEnvelopes{Envelopes: make(map[[32]byte]*Envelope)},

		peers:         make(map[*Peer]struct{}),
		messageQueue:  make(chan *Envelope, messageQueueLimit),
		//p2pMsgQueue:   make(chan *Envelope, messageQueueLimit),
	}

	PeerChannels = make(map[string]chan *gossiper.WhisperPacket)
	whisper.filters = NewFilters(whisper)

	whisper.parameters.Store("pow", DefaultMinimumPoW)

	return whisper
}


// process tlc message
func (whisper *Whisper) processWhisperPacket() {
	for extPacket := range gossiper.PacketChannels["whisperPacket"] {

		// handle gossip message
		if extPacket.Packet.WhisperPacket.Code == messagesCode {
			go whisper.gossiper.HandleGossipMessage(extPacket, extPacket.Packet.WhisperPacket.Origin, extPacket.Packet.WhisperPacket.ID)
		}

		fmt.Println("Got whisper packet")
		fmt.Println(extPacket.SenderAddr)

		if _, err := whisper.getPeer(extPacket.SenderAddr.String()); err != nil {
			// Create the new peer and start tracking it
			whisperPeer := newPeer(whisper, extPacket.SenderAddr)

			whisper.peerMu.Lock()
			whisper.peers[whisperPeer] = struct{}{}
			whisper.peerMu.Unlock()

			go whisper.HandlePeer(extPacket.SenderAddr)
		}

		// handle whisper envelope
		go func(e *gossiper.ExtendedGossipPacket) {
			PeerChannels[e.SenderAddr.String()] <- e.Packet.WhisperPacket
		}(extPacket)
	}
}

// GetMinPow returns the PoW value required by this node.
func (whisper *Whisper) GetMinPow() float64 {
	val, loaded := whisper.parameters.Load("pow")
	if !loaded {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

// MinPowTolerance returns the value of minimum PoW which is tolerated for a limited
// time after PoW was changed. If sufficient time have elapsed or no change of PoW
// have ever occurred, the return value will be the same as return value of GetMinPow().
//func (whisper *Whisper) MinPowTolerance() float64 {
//	val, exist := whisper.settings.Load(minPowToleranceIdx)
//	if !exist || val == nil {
//		return DefaultMinimumPoW
//	}
//	return val.(float64)
//}

func (whisper *Whisper) getPeers() []*Peer {
	arr := make([]*Peer, len(whisper.peers))
	i := 0
	whisper.peerMu.Lock()
	for p := range whisper.peers {
		arr[i] = p
		i++
	}
	whisper.peerMu.Unlock()
	return arr
}

// getPeer retrieves peer by ID
func (whisper *Whisper) getPeer(peerID string) (*Peer, error) {
	whisper.peerMu.Lock()
	defer whisper.peerMu.Unlock()
	for p := range whisper.peers {
		id := p.peer.String()
		if peerID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("could not find peer with ID: %x", peerID)
}

// GetBloomFilter returns the aggregated bloom filter for all the topics of interest.
// The nodes are required to send only messages that match the advertised bloom filter.
// If a message does not match the bloom, it will tantamount to spam, and the peer will
// be disconnected.
func (whisper *Whisper) GetBloomFilter() []byte {
	value, loaded := whisper.parameters.Load("bloom")
	if !loaded {
		return nil
	}
	return value.([]byte)
}

// BloomFilterTolerance returns the bloom filter which is tolerated for a limited
// time after new bloom was advertised to the peers. If sufficient time have elapsed
// or no change of bloom filter have ever occurred, the return value will be the same
// as return value of GetBloomFilter().
//func (whisper *Whisper) BloomFilterTolerance() []byte {
//	val, exist := whisper.settings.Load(bloomFilterToleranceIdx)
//	if !exist || val == nil {
//		return nil
//	}
//	return val.([]byte)
//}

// MaxMessageSize returns the maximum accepted message size.
//func (whisper *Whisper) MaxMessageSize() uint32 {
//	val, _ := whisper.settings.Load(maxMsgSizeIdx)
//	return val.(uint32)
//}

//
//// Overflow returns an indication if the message queue is full.
//func (whisper *Whisper) Overflow() bool {
//	val, _ := whisper.settings.Load(overflowIdx)
//	return val.(bool)
//}

//// APIs returns the RPC descriptors the Whisper implementation offers
//func (whisper *Whisper) APIs() []rpc.API {
//	return []rpc.API{
//		{
//			Namespace: ProtocolName,
//			Version:   ProtocolVersionStr,
//			Service:   NewPublicWhisperAPI(whisper),
//			Public:    true,
//		},
//	}
//}

// RegisterServer registers MailServer interface.
// MailServer will process all the incoming messages with p2pRequestCode.
//func (whisper *Whisper) RegisterServer(server MailServer) {
//	whisper.mailServer = server
//}

//// Protocols returns the whisper sub-protocols ran by this particular client.
//func (whisper *Whisper) Protocols() []gossiper.Protocol {
//	return []gossiper.Protocol{whisper.protocol}
//}
//
//// Version returns the whisper sub-protocols version number.
//func (whisper *Whisper) Version() uint {
//	return whisper.protocol.Version
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
		return fmt.Errorf("invalid bloom filter size: %d", len(bloom))
	}

	b := make([]byte, BloomFilterSize)
	copy(b, bloom)

	whisper.parameters.Store("bloom", b)

	arr := whisper.getPeers()
	for _, p := range arr {
		if err := p.sendWhisperPacket(bloomFilterExCode, &bloom); err != nil {
			fmt.Println("Failed to send bloom update to peer ", p.peer.String())
		}
	}

	return nil
}

// SetMinPoW sets the minimal PoW required by this node
func (whisper *Whisper) SetMinPoW(val float64) error {
	if val < 0.0 {
		return fmt.Errorf("invalid Pow: %f", val)
	}

	whisper.parameters.Store("pow", val)

	arr := whisper.getPeers()
	for _, p := range arr {
		if err := p.sendWhisperPacket(powRequirementCode, &val); err != nil {
			fmt.Println("Failed to send pow update to peer ", p.peer.String())
		}
	}

	return nil
}

//// SetMinimumPowTest sets the minimal PoW in test environment
//func (whisper *Whisper) SetMinimumPowTest(val float64) {
//	whisper.settings.Store(minPowIdx, val)
//	whisper.notifyPeersAboutPowRequirementChange(val)
//	whisper.settings.Store(minPowToleranceIdx, val)
//}
//
////SetLightClientMode makes node light client (does not forward any messages)
//func (whisper *Whisper) SetLightClientMode(v bool) {
//	whisper.settings.Store(lightClientModeIdx, v)
//}
//
////LightClientMode indicates is this node is light client (does not forward any messages)
//func (whisper *Whisper) LightClientMode() bool {
//	val, exist := whisper.settings.Load(lightClientModeIdx)
//	if !exist || val == nil {
//		return false
//	}
//	v, ok := val.(bool)
//	return v && ok
//}

//LightClientModeConnectionRestricted indicates that connection to light client in light client mode not allowed
//func (whisper *Whisper) LightClientModeConnectionRestricted() bool {
//	val, exist := whisper.settings.Load(restrictConnectionBetweenLightClientsIdx)
//	if !exist || val == nil {
//		return false
//	}
//	v, ok := val.(bool)
//	return v && ok
//}



//func (whisper *Whisper) notifyPeersAboutBloomFilterChange(bloom []byte) {
//	arr := whisper.getPeers()
//	for _, p := range arr {
//		err := p.notifyAboutBloomFilterChange(bloom)
//		if err != nil {
//			// allow one retry
//			err = p.notifyAboutBloomFilterChange(bloom)
//		}
//		if err != nil {
//			fmt.Println("failed to notify peer about new bloom filter", "peer", p.peer.String(), "error", err)
//		}
//	}
//}

//// AllowP2PMessagesFromPeer marks specific peer trusted,
//// which will allow it to send historic (expired) messages.
//func (whisper *Whisper) AllowP2PMessagesFromPeer(peerID string) error {
//	p, err := whisper.getPeer(peerID)
//	if err != nil {
//		return err
//	}
//	p.trusted = true
//	return nil
//}

// RequestHistoricMessages sends a message with p2pRequestCode to a specific peer,
// which is known to implement MailServer interface, and is supposed to process this
// request and respond with a number of peer-to-peer messages (possibly expired),
// which are not supposed to be forwarded any further.
// The whisper protocol is agnostic of the format and contents of envelope.
//func (whisper *Whisper) RequestHistoricMessages(peerID string, envelope *Envelope) error {
//	p, err := whisper.getPeer(peerID)
//	if err != nil {
//		return err
//	}
//	p.trusted = true
//	return whisper.gossiper.SendWhisperEnvelope(p2pRequestCode, envelope)
//}
//
//// SendP2PMessage sends a peer-to-peer message to a specific peer.
//func (whisper *Whisper) SendP2PMessage(peerID string, envelope *Envelope) error {
//	p, err := whisper.getPeer(peerID)
//	if err != nil {
//		return err
//	}
//	return whisper.SendP2PDirect(p, envelope)
//}

//// SendP2PDirect sends a peer-to-peer message to a specific peer.
//func (whisper *Whisper) SendP2PDirect(peer *Peer, envelope *Envelope) error {
//	return whisper.gossiper.SendWhisperEnvelope(p2pMessageCode, envelope)
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

// updateBloomFilter recalculates the new value of bloom filter,
// and informs the peers if necessary.
func (whisper *Whisper) updateBloomFilter(f *Filter) {
	aggregate := make([]byte, BloomFilterSize)
	for _, t := range f.Topics {
		top := BytesToTopic(t)
		b := TopicToBloom(top)
		aggregate = addBloom(aggregate, b)
	}

	if !BloomFilterMatch(whisper.GetBloomFilter(), aggregate) {
		// existing bloom filter must be updated
		aggregate = addBloom(whisper.GetBloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

// GetFilter returns the filter by id.
func (whisper *Whisper) GetFilter(id string) *Filter {
	return whisper.filters.GetFilter(id)
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
	ok, err := whisper.add(envelope)
	if err == nil && !ok {
		return fmt.Errorf("failed to add envelope")
	}
	return err
}

// Start implements node.Service, starting the background data propagation thread
// of the Whisper protocol.
func (whisper *Whisper) Start() error {
	go whisper.processWhisperPacket()
	go whisper.update()

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

//HandlePeer is called by the underlying P2P layer when the whisper sub-protocol
//connection is negotiated.
func (whisper *Whisper) HandlePeer(peer *net.UDPAddr) error {
	
	fmt.Println("Handle " + peer.String())

	whisperPeer, err := whisper.getPeer(peer.String())
	if err != nil {
		fmt.Println("No such peer")
		return err
	}

	defer func() {
		whisper.peerMu.Lock()
		delete(whisper.peers, whisperPeer)
		whisper.peerMu.Unlock()
	}()

	// Run the peer handshake and state updates
	if err := whisperPeer.handshake(); err != nil {
		fmt.Println("Handshake failed")
		return err
	} else {
		fmt.Println("Handshake ok")
	}
	whisperPeer.start()
	defer whisperPeer.stop()

	loop := whisper.runMessageLoop(whisperPeer)

	fmt.Println("Error")

	fmt.Println(loop)

	return loop
}

// runMessageLoop reads and processes inbound messages directly to merge into client-global state.
func (whisper *Whisper) runMessageLoop(p *Peer) error {
	for packet := range PeerChannels[p.peer.String()] {

		//// fetch the next packet
		//packet, err := rw.ReadMsg()
		//if err != nil {
		//	fmt.Println("message loop", "peer", p.peer.ID(), "err", err)
		//	return err
		//}
		if packet.Size > DefaultMaxMessageSize {
			//fmt.Println("oversized message received", "peer", p.peer.String())
			return fmt.Errorf("oversized message received")
		}

		switch packet.Code {
		case statusCode:
			pow := p.host.GetMinPow()
			bloom := p.host.GetBloomFilter()

			status := &Status{Bloom: bloom, Pow: pow}
			if err := p.sendWhisperPacket(statusCode, status); err != nil {
				fmt.Println("Failed sending status to peer")
			}
			//packetToSend, err := protobuf.Encode(statusStruct)
			//helpers.ErrorCheck(err, false)
			//
			//wPacket := &gossiper.WhisperPacket{Code: statusCode, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: whisper.gossiper.Name, ID: 0,}
			//gossipPacket := &gossiper.GossipPacket{WhisperPacket: wPacket}
			//
			//fmt.Println("Sent to " + p.peer.String())
			//whisper.gossiper.ConnectionHandler.SendPacket(gossipPacket, p.peer)

		case messagesCode:
			// decode the contained envelope

			fmt.Println("Got envelope")

			envelope := &Envelope{}
			if err := packet.DecodePacket(envelope); err != nil {
				fmt.Println("failed to decode envelopes, peer will be disconnected", "peer", p.peer.String(), "err", err)
				return fmt.Errorf("invalid envelopes")
			}

			trouble := false
			_, err := whisper.add(envelope)
			if err != nil {
				trouble = true
				fmt.Println("bad envelope received, peer will be disconnected", "peer", p.peer.String(), "err", err)
			}
			//if cached {
			//	p.mark(envelope)
			//}

			if trouble {
				return fmt.Errorf("invalid envelope")
			}

			//trouble := false
			//for _, env := range envelopes {
			//	cached, err := whisper.add(env, whisper.LightClientMode())
			//	if err != nil {
			//		trouble = true
			//		fmt.Println("bad envelope received, peer will be disconnected", "peer", p.peer.ID(), "err", err)
			//	}
			//	if cached {
			//		p.mark(env)
			//	}
			//}

			//if trouble {
			//	return fmt.Errorf("invalid envelope")
			//}
		case powRequirementCode:
			var i uint64
			err := packet.DecodePacket(&i)
			if err != nil {
				fmt.Println("failed to decode powRequirementCode message, peer will be disconnected", "peer", p.peer.String(), "err", err)
				return fmt.Errorf("invalid powRequirementCode message")
			}
			f := math.Float64frombits(i)
			if math.IsInf(f, 0) || math.IsNaN(f) || f < 0.0 {
				fmt.Println("invalid value in powRequirementCode message, peer will be disconnected", "peer", p.peer.String(), "err", err)
				return fmt.Errorf("invalid value in powRequirementCode message")
			}
			p.powRequirement = f
		case bloomFilterExCode:
			var bloom []byte
			err := packet.DecodePacket(&bloom)
			if err == nil && len(bloom) != BloomFilterSize {
				err = fmt.Errorf("wrong bloom filter size %d", len(bloom))
			}

			if err != nil {
				fmt.Println("failed to decode bloom filter exchange message, peer will be disconnected", "peer", p.peer.String(), "err", err)
				return fmt.Errorf("invalid bloom filter exchange message")
			}
			p.setBloomFilter(bloom)
		default:
			// New message types might be implemented in the future versions of Whisper.
			// For forward compatibility, just ignore.
		}

		//packet.Discard()
	}

	fmt.Println("aoooooooooooooo")

	return nil
}

// add inserts a new envelope into the message pool to be distributed within the
// whisper network. It also inserts the envelope into the expiration pool at the
// appropriate time-stamp. In case of error, connection should be dropped.
// param isP2P indicates whether the message is peer-to-peer (should not be forwarded).
func (whisper *Whisper) add(envelope *Envelope) (bool, error) {
	now := uint32(time.Now().Unix())
	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		// recalculate PoW, adjusted for the time difference, plus one second for latency
		envelope.calculatePoW(sent - now + 1)
	}

	if envelope.Expiry < now {
		fmt.Println("expired envelope dropped", "hash", envelope.Hash())
		return false, nil // drop envelope without error
	}

	if uint32(envelope.size()) > DefaultMaxMessageSize {
		return false, fmt.Errorf("huge messages are not allowed [%x]", envelope.Hash())
	}

	if envelope.PoW() < whisper.GetMinPow() {
		return false, fmt.Errorf("envelope with low PoW received: PoW=%f, hash=[%v]", envelope.PoW(), envelope.Hash())
	}

	if !BloomFilterMatch(whisper.GetBloomFilter(), envelope.Bloom()) {
		return false, fmt.Errorf("envelope does not match bloom filter, hash=[%v], bloom: \n%x \n%x \n%x", envelope.Hash(), whisper.GetBloomFilter(), envelope.Bloom(), envelope.Topic)
	}

	hash := envelope.Hash()

	whisper.envelopes.Mutex.Lock()
	_, loaded := whisper.envelopes.Envelopes[hash]
	if !loaded {
		whisper.envelopes.Envelopes[hash] = envelope
		whisper.postEvent(envelope)
	} else {
		fmt.Println("whisper envelope already present", "hash", envelope.Hash())
	}
	whisper.envelopes.Mutex.Unlock()

	return true, nil
}

// postEvent queues the message for further processing.
func (whisper *Whisper) postEvent(envelope *Envelope) {
	//whisper.checkOverflow()
	whisper.messageQueue <- envelope

}

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

// processQueue delivers the messages to the watchers during the lifetime of the whisper node.
func (whisper *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-whisper.quit:
			return

		case e = <-whisper.messageQueue:
			whisper.filters.NotifyWatchers(e, false)

		//case e = <-whisper.p2pMsgQueue:
		//	whisper.filters.NotifyWatchers(e, true)
		}
	}
}

// update loops until the lifetime of the whisper node, updating its internal
// state by expiring stale messages from the pool.
func (whisper *Whisper) update() {
	// Start a ticker to check for expirations
	expire := time.NewTicker(expirationCycle)

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

// Envelopes retrieves all the messages currently pooled by the node.
func (whisper *Whisper) Envelopes() []*Envelope {
	whisper.envelopes.Mutex.RLock()
	defer whisper.envelopes.Mutex.RUnlock()

	array := make([]*Envelope, 0, len(whisper.envelopes.Envelopes))
	for _, envelope := range whisper.envelopes.Envelopes {
		array = append(array, envelope)
	}
	return array
}

// isEnvelopeCached checks if envelope with specific hash has already been received and cached.
func (whisper *Whisper) isEnvelopeCached(hash [32]byte) bool {
	whisper.envelopes.Mutex.Lock()
	defer whisper.envelopes.Mutex.Unlock()

	_, exist := whisper.envelopes.Envelopes[hash]
	return exist
}

// validateDataIntegrity returns false if the data have the wrong or contains all zeros,
// which is the simplest and the most  bug.
func validateDataIntegrity(k []byte, expectedSize int) bool {
	if len(k) != expectedSize {
		return false
	}
	if expectedSize > 3 && containsOnlyZeros(k) {
		return false
	}
	return true
}

// containsOnlyZeros checks if the data contain only zeros.
func containsOnlyZeros(data []byte) bool {
	for _, b := range data {
		if b != 0 {
			return false
		}
	}
	return true
}

// bytesToUintLittleEndian converts the slice to 64-bit unsigned integer.
func bytesToUintLittleEndian(b []byte) (res uint64) {
	mul := uint64(1)
	for i := 0; i < len(b); i++ {
		res += uint64(b[i]) * mul
		mul *= 256
	}
	return res
}

//// BytesToUintBigEndian converts the slice to 64-bit unsigned integer.
//func BytesToUintBigEndian(b []byte) (res uint64) {
//	for i := 0; i < len(b); i++ {
//		res *= 256
//		res += uint64(b[i])
//	}
//	return res
//}

// GenerateRandomID generates a random string, which is then returned to be used as a key id
func GenerateRandomID() (id string, err error) {
	buf, err := generateSecureRandomData(keyIDSize)
	if err != nil {
		return "", err
	}
	if !validateDataIntegrity(buf, keyIDSize) {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = hex.EncodeToString(buf)
	return id, err
}

