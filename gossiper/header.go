package gossiper

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	mapset "github.com/deckarep/golang-set"
	"github.com/dedis/protobuf"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/crypto/pbkdf2"
	"sync"
	"sync/atomic"
	"time"
)

// flags
var hw1 = true
var hw2 = true
var hw3 = true
var debug = true

var simpleMode = false
var hw3ex2Mode = false
var hw3ex3Mode = false
var hw3ex4Mode = false
var ackAllMode = false

var modeTypes = []string{"simple", "rumor", "status", "private", "dataRequest", "dataReply", "searchRequest", "searchReply", "tlcMes", "tlcAck", "clientBlock", "tlcCausal", "whisperPacket"}

// channels used throgout the app to exchange messages
var PacketChannels map[string]chan *ExtendedGossipPacket

// constants

var maxBufferSize = 60000
var maxChannelSize = 100

// timeouts in seconds if not specified
var rumorTimeout = 10
var stubbornTimeout = 10
var routeRumorTimeout = 0
var antiEntropyTimeout = 10
var requestTimeout = 5
var searchTimeout = 1
var searchRequestDuplicateTimeout = 500 * time.Millisecond
var tlcQueueTimeout = 1

var latestMessagesBuffer = 30
var hopLimit = 10
var matchThreshold = 2
var maxBudget = 32
var defaultBudget = 2

const fileChunk = 8192

var shareFolder = "/_SharedFiles/"
var downloadFolder = "/_Downloads/"

// SimpleMessage struct
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

// GossipPacket struct
type GossipPacket struct {
	Simple        *SimpleMessage
	Rumor         *RumorMessage
	Status        *StatusPacket
	Private       *PrivateMessage
	DataRequest   *DataRequest
	DataReply     *DataReply
	SearchRequest *SearchRequest
	SearchReply   *SearchReply
	TLCMessage    *TLCMessage
	Ack           *TLCAck
	WhisperPacket *WhisperPacket
}

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

// PrivateMessage struct
type PrivateMessage struct {
	Origin      string
	ID          uint32
	Text        string
	Destination string
	HopLimit    uint32
}

// DataRequest struct
type DataRequest struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
}

// DataReply struct
type DataReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	HashValue   []byte
	Data        []byte
}

// SearchRequest struct
type SearchRequest struct {
	Origin   string
	Budget   uint64
	Keywords []string
}

// SearchReply struct
type SearchReply struct {
	Origin      string
	Destination string
	HopLimit    uint32
	Results     []*SearchResult
}

// SearchResult struct
type SearchResult struct {
	FileName     string
	MetafileHash []byte
	ChunkMap     []uint64
	ChunkCount   uint64
}

// TxPublish struct
type TxPublish struct {
	Name         string
	Size         int64
	MetafileHash []byte
}

// BlockPublish struct
type BlockPublish struct {
	PrevHash    [32]byte
	Transaction TxPublish
}

// TLCMessage struct
type TLCMessage struct {
	Origin      string
	ID          uint32
	Confirmed   int
	TxBlock     BlockPublish
	VectorClock *StatusPacket
	Fitness     float32
}

// TLCAck type
type TLCAck PrivateMessage

// WhisperPacket struct
type WhisperPacket struct {
	Origin string
	ID 	uint32
	Code  	uint
	Size 	uint32
	Payload []byte
}

const (
	maxMsgSizeIdx                            = iota // Maximal message length allowed by the whisper node
	overflowIdx                                     // Indicator of message queue overflow
	minPowIdx                                       // Minimal PoW required by the whisper node
	bloomFilterIdx                                  // Bloom filter for topics of interest for this node
)

// Whisper data structure
type Whisper struct {
	filters  *Filters  // Message filters installed with Subscribe function

	privateKeys map[string]*ecdsa.PrivateKey // Private key storage
	symKeys     map[string][]byte            // Symmetric key storage
	keyMu       sync.RWMutex                 // Mutex associated with key storages

	poolMu      sync.RWMutex                        // Mutex to sync the message and expiration pools
	envelopes   map[common.Hash]*Envelope // Pool of envelopes currently tracked by this node
	expirations map[uint32]mapset.Set               // Message expiration pool

	settings sync.Map // holds configuration settings that can be dynamically changed
}

// New creates a Whisper client ready to communicate through the Ethereum P2P network.
func NewWhisper() *Whisper {

	whisper := &Whisper{
		privateKeys: make(map[string]*ecdsa.PrivateKey),
		symKeys:     make(map[string][]byte),
		envelopes:   make(map[common.Hash]*Envelope),
		expirations: make(map[uint32]mapset.Set),
	}

	whisper.filters = NewFilters(whisper)

	whisper.settings.Store(minPowIdx, DefaultMinimumPoW)
	whisper.settings.Store(maxMsgSizeIdx, DefaultMaxMessageSize)
	whisper.settings.Store(overflowIdx, false)

	return whisper
}

func (wp *WhisperPacket) DecodeEnvelope(envelope *Envelope) error {

	whisperEnvelope := &Envelope{}
	packetBytes := make([]byte, maxBufferSize)

	// decode message
	err := protobuf.Decode(packetBytes[:wp.Size], whisperEnvelope)
	if err != nil {
		return err
	}

	envelope = whisperEnvelope

	return nil
}

func (wp *WhisperPacket) DecodeBloom(bloom *[]byte) error {

	bloomFilter := &[]byte{}
	packetBytes := make([]byte, maxBufferSize)

	// decode message
	err := protobuf.Decode(packetBytes[:wp.Size], bloomFilter)
	if err != nil {
		return err
	}

	bloom = bloomFilter

	return nil
}

func (gossiper *Gossiper) SendWhisperEnvelope(code uint, envelope *Envelope) error {

	packetToSend, err := protobuf.Encode(envelope)
	if err != nil {
		return err
	}

	id := atomic.LoadUint32(&gossiper.gossipHandler.seqID)
	atomic.AddUint32(&gossiper.gossipHandler.seqID, uint32(1))

	wPacket := &WhisperPacket{Code: code, Payload: packetToSend, Size: uint32(len(packetToSend)), Origin: gossiper.name, ID: id,}
	extPacket := &ExtendedGossipPacket{SenderAddr: gossiper.connectionHandler.gossiperData.Address, Packet: &GossipPacket{WhisperPacket: wPacket}}

	// store message
	gossiper.gossipHandler.storeMessage(extPacket.Packet, gossiper.name, id)

	// start rumor mongering the message
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)

	return nil
}

// MinPow returns the PoW value required by this node.
func (whisper *Whisper) MinPow() float64 {
	val, exist := whisper.settings.Load(minPowIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	v, ok := val.(float64)
	if !ok {
		log.Error("Error loading minPowIdx, using default")
		return DefaultMinimumPoW
	}
	return v
}

// BloomFilter returns the aggregated bloom filter for all the topics of interest.
// The nodes are required to send only messages that match the advertised bloom filter.
// If a message does not match the bloom, it will tantamount to spam, and the peer will
// be disconnected.
func (whisper *Whisper) BloomFilter() []byte {
	val, exist := whisper.settings.Load(bloomFilterIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

// MaxMessageSize returns the maximum accepted message size.
func (whisper *Whisper) MaxMessageSize() uint32 {
	val, _ := whisper.settings.Load(maxMsgSizeIdx)
	return val.(uint32)
}

// SetMaxMessageSize sets the maximal message size allowed by this node
func (whisper *Whisper) SetMaxMessageSize(size uint32) error {
	if size > MaxMessageSize {
		return fmt.Errorf("message size too large [%d>%d]", size, MaxMessageSize)
	}
	whisper.settings.Store(maxMsgSizeIdx, size)
	return nil
}

// SetBloomFilter sets the new bloom filter
func (whisper *Whisper) SetBloomFilter(bloom []byte) error {
	if len(bloom) != BloomFilterSize {
		return fmt.Errorf("invalid bloom filter size: %d", len(bloom))
	}

	b := make([]byte, BloomFilterSize)
	copy(b, bloom)

	whisper.settings.Store(bloomFilterIdx, b)

	return nil
}

// SetMinimumPoW sets the minimal PoW required by this node
func (whisper *Whisper) SetMinimumPoW(val float64) error {
	if val < 0.0 {
		return fmt.Errorf("invalid PoW: %f", val)
	}

	whisper.settings.Store(minPowIdx, val)
	//whisper.notifyPeersAboutPowRequirementChange(val)

	return nil
}

// NewKeyPair generates a new cryptographic identity for the client, and injects
// it into the known identities for message decryption. Returns ID of the new key pair.
func (whisper *Whisper) NewKeyPair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil || !validatePrivateKey(key) {
		key, err = crypto.GenerateKey() // retry once
	}
	if err != nil {
		return "", err
	}
	if !validatePrivateKey(key) {
		return "", fmt.Errorf("failed to generate valid key")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.privateKeys[id] = key
	return id, nil
}

// DeleteKeyPair deletes the specified key if it exists.
func (whisper *Whisper) DeleteKeyPair(key string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.privateKeys[key] != nil {
		delete(whisper.privateKeys, key)
		return true
	}
	return false
}

// AddKeyPair imports a asymmetric private key and returns it identifier.
func (whisper *Whisper) AddKeyPair(key *ecdsa.PrivateKey) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	whisper.privateKeys[id] = key
	whisper.keyMu.Unlock()

	return id, nil
}

// HasKeyPair checks if the whisper node is configured with the private key
// of the specified public pair.
func (whisper *Whisper) HasKeyPair(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.privateKeys[id] != nil
}

// GetPrivateKey retrieves the private key of the specified identity.
func (whisper *Whisper) GetPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	key := whisper.privateKeys[id]
	if key == nil {
		return nil, fmt.Errorf("invalid id")
	}
	return key, nil
}

// GenerateSymKey generates a random symmetric key and stores it under id,
// which is then returned. Will be used in the future for session key exchange.
func (whisper *Whisper) GenerateSymKey() (string, error) {
	key, err := generateSecureRandomData(aesKeyLength)
	if err != nil {
		return "", err
	} else if !validateDataIntegrity(key, aesKeyLength) {
		return "", fmt.Errorf("error in GenerateSymKey: crypto/rand failed to generate random data")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

// AddSymKeyDirect stores the key, and returns its id.
func (whisper *Whisper) AddSymKeyDirect(key []byte) (string, error) {
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key size: %d", len(key))
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	whisper.symKeys[id] = key
	return id, nil
}

// AddSymKeyFromPassword generates the key from password, stores it, and returns its id.
func (whisper *Whisper) AddSymKeyFromPassword(password string) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}
	if whisper.HasSymKey(id) {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	// kdf should run no less than 0.1 seconds on an average computer,
	// because it's an once in a session experience
	derived := pbkdf2.Key([]byte(password), nil, 65356, aesKeyLength, sha256.New)
	if err != nil {
		return "", err
	}

	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()

	// double check is necessary, because deriveKeyMaterial() is very slow
	if whisper.symKeys[id] != nil {
		return "", fmt.Errorf("critical error: failed to generate unique ID")
	}
	whisper.symKeys[id] = derived
	return id, nil
}

// HasSymKey returns true if there is a key associated with the given id.
// Otherwise returns false.
func (whisper *Whisper) HasSymKey(id string) bool {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	return whisper.symKeys[id] != nil
}

// DeleteSymKey deletes the key associated with the name string if it exists.
func (whisper *Whisper) DeleteSymKey(id string) bool {
	whisper.keyMu.Lock()
	defer whisper.keyMu.Unlock()
	if whisper.symKeys[id] != nil {
		delete(whisper.symKeys, id)
		return true
	}
	return false
}

// GetSymKey returns the symmetric key associated with the given id.
func (whisper *Whisper) GetSymKey(id string) ([]byte, error) {
	whisper.keyMu.RLock()
	defer whisper.keyMu.RUnlock()
	if whisper.symKeys[id] != nil {
		return whisper.symKeys[id], nil
	}
	return nil, fmt.Errorf("non-existent key ID")
}

// Subscribe installs a new message handler used for filtering, decrypting
// and subsequent storing of incoming messages.
func (whisper *Whisper) Subscribe(f *Filter) (string, error) {
	s, err := whisper.filters.Install(f)
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

	if !BloomFilterMatch(whisper.BloomFilter(), aggregate) {
		// existing bloom filter must be updated
		aggregate = addBloom(whisper.BloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

// GetFilter returns the filter by id.
func (whisper *Whisper) GetFilter(id string) *Filter {
	return whisper.filters.Get(id)
}

// Unsubscribe removes an installed message handler.
func (whisper *Whisper) Unsubscribe(id string) error {
	ok := whisper.filters.Uninstall(id)
	if !ok {
		return fmt.Errorf("Unsubscribe: Invalid ID")
	}
	return nil
}

//// Send injects a message into the whisper send queue, to be distributed in the
//// network in the coming cycles.
//func (whisper *Whisper) Send(envelope *Envelope) error {
//	ok, err := whisper.add(envelope)
//	if err == nil && !ok {
//		return fmt.Errorf("failed to add envelope")
//	}
//	return err
//}

//// Start implements node.Service, starting the background data propagation thread
//// of the Whisper protocol.
//func (whisper *Whisper) Start() error {
//	go whisper.update()
//
//	numCPU := runtime.NumCPU()
//	for i := 0; i < numCPU; i++ {
//		go whisper.processQueue()
//	}
//
//	return nil
//}

//// Stop implements node.Service, stopping the background data propagation thread
//// of the Whisper protocol.
//func (whisper *Whisper) Stop() error {
//	close(whisper.quit)
//	log.Info("whisper stopped")
//	return nil
//}

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
		}
	}
}

// expire iterates over all the expiration timestamps, removing all stale
// messages from the pools.
func (whisper *Whisper) expire() {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	now := uint32(time.Now().Unix())
	for expiry, hashSet := range whisper.expirations {
		if expiry < now {
			// Dump all expired messages and remove timestamp
			hashSet.Each(func(v interface{}) bool {
				delete(whisper.envelopes, v.(common.Hash))
				return false
			})
			whisper.expirations[expiry].Clear()
			delete(whisper.expirations, expiry)
		}
	}
}

// Envelopes retrieves all the messages currently pooled by the node.
func (whisper *Whisper) Envelopes() []*Envelope {
	whisper.poolMu.RLock()
	defer whisper.poolMu.RUnlock()

	all := make([]*Envelope, 0, len(whisper.envelopes))
	for _, envelope := range whisper.envelopes {
		all = append(all, envelope)
	}
	return all
}

// isEnvelopeCached checks if envelope with specific hash has already been received and cached.
func (whisper *Whisper) isEnvelopeCached(hash common.Hash) bool {
	whisper.poolMu.Lock()
	defer whisper.poolMu.Unlock()

	_, exist := whisper.envelopes[hash]
	return exist
}

// ValidatePublicKey checks the format of the given public key.
func ValidatePublicKey(k *ecdsa.PublicKey) bool {
	return k != nil && k.X != nil && k.Y != nil && k.X.Sign() != 0 && k.Y.Sign() != 0
}

// validatePrivateKey checks the format of the given private key.
func validatePrivateKey(k *ecdsa.PrivateKey) bool {
	if k == nil || k.D == nil || k.D.Sign() == 0 {
		return false
	}
	return ValidatePublicKey(&k.PublicKey)
}

// validateDataIntegrity returns false if the data have the wrong or contains all zeros,
// which is the simplest and the most common bug.
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

// BytesToUintBigEndian converts the slice to 64-bit unsigned integer.
func BytesToUintBigEndian(b []byte) (res uint64) {
	for i := 0; i < len(b); i++ {
		res *= 256
		res += uint64(b[i])
	}
	return res
}

// GenerateRandomID generates a random string, which is then returned to be used as a key id
func GenerateRandomID() (id string, err error) {
	buf, err := generateSecureRandomData(keyIDSize)
	if err != nil {
		return "", err
	}
	if !validateDataIntegrity(buf, keyIDSize) {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = common.Bytes2Hex(buf)
	return id, err
}

func isFullNode(bloom []byte) bool {
	if bloom == nil {
		return true
	}
	for _, b := range bloom {
		if b != 255 {
			return false
		}
	}
	return true
}

func BloomFilterMatch(filter, sample []byte) bool {
	if filter == nil {
		return true
	}

	for i := 0; i < BloomFilterSize; i++ {
		f := filter[i]
		s := sample[i]
		if (f | s) != f {
			return false
		}
	}

	return true
}

func addBloom(a, b []byte) []byte {
	c := make([]byte, BloomFilterSize)
	for i := 0; i < BloomFilterSize; i++ {
		c[i] = a[i] | b[i]
	}
	return c
}
