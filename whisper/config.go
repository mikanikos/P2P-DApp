package whisper

import (
	"time"
)

// main whisper protocol parameters, from official source
const (
	statusCode           = 0
	messagesCode         = 1
	powRequirementCode   = 2
	bloomFilterExCode    = 3

	// lengths in bytes
	TopicLength     = 4
	aesKeyLength    = 32
	keyIDSize       = 32
	BloomFilterSize = 64
	flagsLength     = 1

	EnvelopeHeaderLength = 20

	MaxMessageSize        = uint32(10 * 1024 * 1024) // maximum accepted size of a message.
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2

	padSizeLimit      = 256
	messageQueueLimit = 1024

	expirationCycle   = time.Second
	transmissionCycle = 5 * time.Second

	DefaultTTL = 50 // in seconds
)
