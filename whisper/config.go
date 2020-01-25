package whisper

import (
	"time"
)

// main whisper protocol parameters, from official source
const (
	statusCode         = 0
	messagesCode       = 1
	powRequirementCode = 2
	bloomFilterExCode  = 3

	// lengths in bytes
	TopicLength     = 4
	aesKeyLength    = 32
	keyIDSize       = 32
	BloomFilterSize = 64

	MaxMessageSize        = uint32(10 * 1024 * 1024) // maximum accepted size of a message.
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2
	DefaultTTL            = 50 // in seconds
	DefaultSyncAllowance  = 10 // seconds

	padSizeLimit      = 256
	messageQueueLimit = 1024

	expirationTimer = 3 * time.Second
	broadcastTimer  = time.Second
	statusTimer     = 5 * time.Second
)

const (
	maxMsgSizeIdx           = iota // Maximal message length allowed by the whisper node
	minPowIdx                      // Minimal PoW required by the whisper node
	minPowToleranceIdx             // Minimal PoW tolerated by the whisper node for a limited time
	bloomFilterIdx                 // Bloom filter for topics of interest for this node
	bloomFilterToleranceIdx        // Bloom filter tolerated by the whisper node for a limited time
)
