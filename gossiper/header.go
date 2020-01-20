package gossiper

import (
	"github.com/dedis/protobuf"
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