package gossiper

import (
	"net"
	"time"
)

var hw1 = false
var hw2 = true
var hw3 = true
var debug = true

var simpleMode = false
var hw3ex2Mode = false
var hw3ex3Mode = false
var hw3ex4Mode = false
var ackAllMode = false

var modeTypes = []string{"simple", "rumor", "status", "private", "dataRequest", "dataReply", "searchRequest", "searchReply", "tlcMes", "tlcAck"}

var maxBufferSize = 60000

var rumorTimeout = 10
var stubbornTimeout = 10
var requestTimeout = 5
var searchTimeout = 1
var searchRequestDuplicateTimeout = 500 * time.Millisecond

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
}

// NetworkData struct
type NetworkData struct {
	Conn *net.UDPConn
	Addr *net.UDPAddr
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
	Size         int64 // Size in bytes
	MetafileHash []byte
}

// BlockPublish struct
type BlockPublish struct {
	PrevHash    [32]byte // (used in Exercise 4, for now 0)
	Transaction TxPublish
}

// TLCMessage struct
type TLCMessage struct {
	Origin      string
	ID          uint32
	Confirmed   bool
	TxBlock     BlockPublish
	VectorClock *StatusPacket // (used in Exercise 3, for now nil)
	Fitness     float32       // (used in Exercise 4, for now 0)
}

// TLCAck type
type TLCAck PrivateMessage
