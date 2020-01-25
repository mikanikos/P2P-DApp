package gossiper

import (
	"sync/atomic"
	"time"
)

// flags
var hw1 = false
var hw2 = false
var hw3 = false
var debug = false

var simpleMode = false
var hw3ex2Mode = false
var hw3ex3Mode = false
var hw3ex4Mode = false
var ackAllMode = false

var modeTypes = []string{"simple", "rumor", "status", "private", "dataRequest", "dataReply", "searchRequest", "searchReply", "tlcMes", "tlcAck", "clientBlock", "tlcCausal", "whisperPacket", "whisperStatus"}

// channels used throughout the app to exchange messages
var PacketChannels map[string]chan *ExtendedGossipPacket

// constants

var maxBufferSize = 60000
var maxChannelSize = 1024

// timeouts in seconds if not specified
var rumorTimeout = 1
var stubbornTimeout = 10
var routeRumorTimeout = 0
var antiEntropyTimeout = 1
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
	WhisperStatus *WhisperStatus
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
	Code    uint32
	Size    uint32
	Payload []byte
}

// WhisperStatus struct
type WhisperStatus struct {
	Origin string
	ID     uint32
	Code   uint32
	Bloom  []byte
	Pow    float64
}

//func (wp *WhisperPacket) DecodePacket(envelope *whisper.Envelope) error {
//
//	// decode message
//	err := protobuf.Decode(wp.Payload, envelope)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return err
//}
//
//func (wp *WhisperPacket) DecodePacket(pow *uint64) error {
//
//	// decode message
//	err := protobuf.Decode(wp.Payload, pow)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return err
//}
//
//func (wp *WhisperPacket) DecodePacket(bloom *[]byte) error {
//
//	// decode message
//	err := protobuf.Decode(wp.Payload, bloom)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return err
//}
//
//func (wp *WhisperPacket) DecodePacket(status *whisper.WhisperStatus) error {
//
//	//packetBytes := make([]byte, maxBufferSize)
//	// decode message
//	err := protobuf.Decode(wp.Payload, status)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return err
//}

//func (wp *WhisperPacket) DecodePacket(int interface{}) error {
//
//	// decode message
//	err := protobuf.Decode(wp.Payload, int)
//	if err != nil {
//		fmt.Println(err)
//	}
//
//	return err
//}

func (gossiper *Gossiper) SendWhisperStatus(status *WhisperStatus) {

	id := atomic.LoadUint32(&gossiper.gossipHandler.seqID)
	atomic.AddUint32(&gossiper.gossipHandler.seqID, uint32(1))

	status.Origin = gossiper.Name
	status.ID = id

	packet := &GossipPacket{WhisperStatus: status}

	// store message
	gossiper.gossipHandler.storeMessage(packet, gossiper.Name, id)

	go gossiper.startRumorMongering(&ExtendedGossipPacket{SenderAddr: gossiper.ConnectionHandler.gossiperData.Address, Packet: packet}, gossiper.Name, id)
}

//
//func (gossiper *Gossiper) CreateWhisperPacket(code uint32, data interface{}) *WhisperStatus {
//
//	wPacket := &WhisperStatus{Code: code, Payload: payload, Size: uint32(len(payload)), Origin: gossiper.Name, ID: 0,}
//	return &ExtendedGossipPacket{SenderAddr: gossiper.ConnectionHandler.gossiperData.Address, Packet: &GossipPacket{WhisperPacket: wPacket}}
//}
