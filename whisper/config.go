package whisper

import (
	"github.com/mikanikos/Peerster/crypto"
	"time"
)

// Whisper protocol parameters
const (
	// whisper protocol message codes, according to EIP-627
	statusCode           = 0   // used by whisper protocol
	messagesCode         = 1   // normal whisper message
	powRequirementCode   = 2   // PoW requirement
	bloomFilterExCode    = 3   // bloom filter exchange

	SizeMask      = byte(3) // mask used to extract the size of payload size field from the flags
	signatureFlag = byte(4)

	TopicLength     = 4                      // in bytes
	signatureLength = crypto.SignatureLength // in bytes
	aesKeyLength    = 32                     // in bytes
	aesNonceLength  = 12                     // in bytes; for more info please see cipher.gcmStandardNonceSize & aesgcm.NonceSize()
	keyIDSize       = 32                     // in bytes
	BloomFilterSize = 64                     // in bytes
	flagsLength     = 1

	EnvelopeHeaderLength = 20

	MaxMessageSize        = uint32(10 * 1024 * 1024) // maximum accepted size of a message.
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2

	padSizeLimit      = 256 // just an arbitrary number, could be changed without breaking the protocol
	messageQueueLimit = 1024

	expirationCycle   = time.Second
	transmissionCycle = 5 * time.Second

	DefaultTTL           = 50 // seconds
)
