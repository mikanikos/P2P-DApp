package whisper

import (
	"github.com/dedis/protobuf"
	ecies "github.com/ecies/go"
	"golang.org/x/crypto/sha3"
	"time"
)

// Envelope according to Whisper protocol
type Envelope struct {
	Expiry uint32
	TTL    uint32
	Topic  Topic
	Data   []byte
	Nonce  uint64

	pow   float64
	bloom []byte
}

// NewEnvelope creates new envelope
func NewEnvelope(ttl uint32, topic Topic, data []byte) *Envelope {
	return &Envelope{
		Expiry: uint32(time.Now().Add(time.Duration(ttl) * time.Second).Unix()),
		TTL:    ttl,
		Topic:  topic,
		Data:   data,
		Nonce:  0,
	}
}

//size returns the size of envelope
func (e *Envelope) size() int {
	encodedEnvelope, _ := protobuf.Encode(e)
	return len(encodedEnvelope)
}

// GetPow compute pow if not already done
func (e *Envelope) GetPow() float64 {
	if e.pow == 0 {
		e.computePow(0)
	}
	return e.pow
}

// GetBloom compute bloom if not already done
func (e *Envelope) GetBloom() []byte {
	if e.bloom == nil {
		e.bloom = ConvertTopicToBloom(e.Topic)
	}
	return e.bloom
}

// Hash returns the SHA3 hash of the envelope, calculating it if not yet done.
func (e *Envelope) Hash() [32]byte {
	encoded, _ := protobuf.Encode(e)
	return sha3.Sum256(encoded)
}

// OpenWithPrivateKey decrypts an envelope with an asymmetric key
func (e *Envelope) OpenWithPrivateKey(key *ecies.PrivateKey) (*ReceivedMessage, error) {
	decrypted, err := decryptWithPrivateKey(e.Data, key)
	if err != nil {
		return nil, err
	}
	return &ReceivedMessage{Payload: decrypted}, nil
}

// OpenWithSymmetricKey decrypts an envelope with a symmetric key
func (e *Envelope) OpenWithSymmetricKey(key []byte) (*ReceivedMessage, error) {
	decrypted, err := decryptWithSymmetricKey(e.Data, key)
	if err != nil {
		return nil, err
	}
	return &ReceivedMessage{Payload: decrypted}, nil
}

// GetMessageFromEnvelope decrypts the message payload of the envelope
func (e *Envelope) GetMessageFromEnvelope(subscriber *Filter) *ReceivedMessage {
	if subscriber == nil {
		return nil
	}

	if subscriber.isAsymmetricEncryption() {
		msg, err := e.OpenWithPrivateKey(subscriber.KeyAsym)
		if err != nil {
			return nil
		}
		if msg != nil {
			msg.Dst = subscriber.KeyAsym.PublicKey
		}
	} else if subscriber.isSymmetricEncryption() {
		msg, err := e.OpenWithSymmetricKey(subscriber.KeySym)
		if err != nil {
			return nil
		}

		if msg != nil {
			msg.SymKeyHash = sha3.Sum256(subscriber.KeySym)
		}
	}

	return &ReceivedMessage{
		Topic:        e.Topic,
		TTL:          e.TTL,
		Sent:         e.Expiry - e.TTL,
		EnvelopeHash: e.Hash(),
	}
}
