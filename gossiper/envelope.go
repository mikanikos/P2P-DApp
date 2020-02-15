package gossiper

import (
	"crypto/ecdsa"
	"encoding/binary"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/whisper"
	gmath "math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

// Envelope represents a clear-text data packet to transmit through the Whisper
// network. Its contents may or may not be encrypted and signed.
type Envelope struct {
	Expiry uint32
	TTL    uint32
	Topic  TopicType
	Data   []byte
	Nonce  uint64

	pow float64 // Message-specific PoW as described in the Whisper specification.

	// the following variables should not be accessed directly, use the corresponding function instead: Hash(), Bloom()
	hash  common.Hash // Cached hash of the envelope to avoid rehashing every time.
	bloom []byte
}

// size returns the size of envelope as it is sent (i.e. public fields only)
func (e *Envelope) size() int {
	return EnvelopeHeaderLength + len(e.Data)
}

// EncodeWithoutNonce returns the RLP encoded envelope contents, except the nonce.
func (e *Envelope) EncodeWithoutNonce() []byte {
	res, _ := protobuf.Encode([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	return res
}

// NewEnvelope wraps a Whisper message with expiration and destination data
// included into an envelope for network forwarding.
func NewEnvelope(ttl uint32, topic TopicType, msg *whisper.sentMessage) *Envelope {
	env := Envelope{
		Expiry: uint32(time.Now().Add(time.Second * time.Duration(ttl)).Unix()),
		TTL:    ttl,
		Topic:  topic,
		Data:   msg.Raw,
		Nonce:  0,
	}

	return &env
}

// Seal closes the envelope by spending the requested amount of time as a proof
// of work on hashing the data.
func (e *Envelope) Seal(options *MessageParams) error {
	if options.PoW == 0 {
		// PoW is not required
		return nil
	}

	var target, bestLeadingZeros int
	if options.PoW < 0 {
		e.Expiry += options.WorkTime
	} else {
		target = e.powToFirstBit(options.PoW)
	}

	rlp := e.EncodeWithoutNonce()
	buf := make([]byte, len(rlp)+8)
	copy(buf, rlp)
	asAnInt := new(big.Int)

	finish := time.Now().Add(time.Duration(options.WorkTime) * time.Second).UnixNano()
	for nonce := uint64(0); time.Now().UnixNano() < finish; {
		for i := 0; i < 1024; i++ {
			binary.BigEndian.PutUint64(buf[len(rlp):], nonce)
			h := crypto.Keccak256(buf)
			asAnInt.SetBytes(h)
			leadingZeros := 256 - asAnInt.BitLen()
			if leadingZeros > bestLeadingZeros {
				e.Nonce, bestLeadingZeros = nonce, leadingZeros
				if target > 0 && bestLeadingZeros >= target {
					return nil
				}
			}
			nonce++
		}
	}

	if target > 0 && bestLeadingZeros < target {
		return fmt.Errorf("failed to reach the PoW target, specified pow time (%d seconds) was insufficient", options.WorkTime)
	}

	return nil
}

// PoW computes (if necessary) and returns the proof of work target
// of the envelope.
func (e *Envelope) PoW() float64 {
	if e.pow == 0 {
		e.calculatePoW(0)
	}
	return e.pow
}

func (e *Envelope) calculatePoW(diff uint32) {
	rlp := e.EncodeWithoutNonce()
	buf := make([]byte, len(rlp)+8)
	copy(buf, rlp)
	binary.BigEndian.PutUint64(buf[len(rlp):], e.Nonce)
	powHash := new(big.Int).SetBytes(crypto.Keccak256(buf))
	leadingZeroes := 256 - powHash.BitLen()
	x := gmath.Pow(2, float64(leadingZeroes))
	x /= float64(len(rlp))
	x /= float64(e.TTL + diff)
	e.pow = x
}

func (e *Envelope) powToFirstBit(pow float64) int {
	x := pow
	x *= float64(e.size())
	x *= float64(e.TTL)
	bits := gmath.Log2(x)
	bits = gmath.Ceil(bits)
	res := int(bits)
	if res < 1 {
		res = 1
	}
	return res
}

// Hash returns the SHA3 hash of the envelope, calculating it if not yet done.
func (e *Envelope) Hash() common.Hash {
	if (e.hash == common.Hash{}) {
		encoded, _ := protobuf.Encode(e)
		e.hash = crypto.Keccak256Hash(encoded)
	}
	return e.hash
}

// OpenAsymmetric tries to decrypt an envelope, potentially encrypted with a particular key.
func (e *Envelope) OpenAsymmetric(key *ecdsa.PrivateKey) (*ReceivedMessage, error) {
	message := &ReceivedMessage{Raw: e.Data}
	err := message.decryptAsymmetric(key)
	switch err {
	case nil:
		return message, nil
	case ecies.ErrInvalidPublicKey: // addressed to somebody else
		return nil, err
	default:
		return nil, fmt.Errorf("unable to open envelope, decrypt failed: %v", err)
	}
}

// OpenSymmetric tries to decrypt an envelope, potentially encrypted with a particular key.
func (e *Envelope) OpenSymmetric(key []byte) (msg *ReceivedMessage, err error) {
	msg = &ReceivedMessage{Raw: e.Data}
	err = msg.decryptSymmetric(key)
	if err != nil {
		msg = nil
	}
	return msg, err
}

// Open tries to decrypt an envelope, and populates the message fields in case of success.
func (e *Envelope) Open(watcher *Filter) (msg *ReceivedMessage) {
	if watcher == nil {
		return nil
	}

	// The API interface forbids filters doing both symmetric and asymmetric encryption.
	if watcher.expectsAsymmetricEncryption() && watcher.expectsSymmetricEncryption() {
		return nil
	}

	if watcher.expectsAsymmetricEncryption() {
		msg, _ = e.OpenAsymmetric(watcher.KeyAsym)
		if msg != nil {
			msg.Dst = &watcher.KeyAsym.PublicKey
		}
	} else if watcher.expectsSymmetricEncryption() {
		msg, _ = e.OpenSymmetric(watcher.KeySym)
		if msg != nil {
			msg.SymKeyHash = crypto.Keccak256Hash(watcher.KeySym)
		}
	}

	if msg != nil {
		ok := msg.ValidateAndParse()
		if !ok {
			return nil
		}
		msg.Topic = e.Topic
		msg.PoW = e.PoW()
		msg.TTL = e.TTL
		msg.Sent = e.Expiry - e.TTL
		msg.EnvelopeHash = e.Hash()
	}
	return msg
}

// Bloom maps 4-bytes Topic into 64-byte bloom filter with 3 bits set (at most).
func (e *Envelope) Bloom() []byte {
	if e.bloom == nil {
		e.bloom = TopicToBloom(e.Topic)
	}
	return e.bloom
}

// TopicToBloom converts the topic (4 bytes) to the bloom filter (64 bytes)
func TopicToBloom(topic TopicType) []byte {
	b := make([]byte, BloomFilterSize)
	var index [3]int
	for j := 0; j < 3; j++ {
		index[j] = int(topic[j])
		if (topic[3] & (1 << uint(j))) != 0 {
			index[j] += 256
		}
	}

	for j := 0; j < 3; j++ {
		byteIndex := index[j] / 8
		bitIndex := index[j] % 8
		b[byteIndex] = (1 << uint(bitIndex))
	}
	return b
}

// GetEnvelope retrieves an envelope from the message queue by its hash.
// It returns nil if the envelope can not be found.
func (w *whisper.Whisper) GetEnvelope(hash common.Hash) *Envelope {
	w.poolMu.RLock()
	defer w.poolMu.RUnlock()
	return w.envelopes[hash]
}
