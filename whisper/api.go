// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package whisper

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/mikanikos/Peerster/crypto"
)

// List of errors
var (
	ErrSymAsym              = fmt.Errorf("specify either a symmetric or an asymmetric key")
	ErrInvalidSymmetricKey  = fmt.Errorf("invalid symmetric key")
	ErrInvalidPublicKey     = fmt.Errorf("invalid public key")
	ErrInvalidSigningPubKey = fmt.Errorf("invalid signing public key")
	ErrTooLowPoW            = fmt.Errorf("message rejected, PoW too low")
	ErrNoTopics             = fmt.Errorf("missing topic(s)")
)

// PublicWhisperAPI provides the whisper RPC service that can be
// use publicly without security implications.
//type PublicWhisperAPI struct {
//	w *Whisper
//
//	mu       sync.Mutex
//	lastUsed map[string]time.Time // keeps track when a filter was polled for the last time.
//}

// NewPublicWhisperAPI create a new RPC whisper service.
//func NewPublicWhisperAPI(w *Whisper) *PublicWhisperAPI {
//	api := &PublicWhisperAPI{
//		w:        w,
//		lastUsed: make(map[string]time.Time),
//	}
//	return api
//}

//// Version returns the Whisper sub-protocol version.
//func (whisper *Whisper) Version() string {
//	return ProtocolVersionStr
//}

// Info contains diagnostic information.
//type Info struct {
//	Memory         int     `json:"memory"`         // Memory size of the floating messages in bytes.
//	Messages       int     `json:"messages"`       // Number of floating messages.
//	GetMinPow         float64 `json:"minPow"`         // Minimal accepted PoW
//	MaxMessageSize uint32  `json:"maxMessageSize"` // Maximum accepted message size
//}

//// Info returns diagnostic information about the whisper node.
//func (whisper *Whisper) Info() Info {
//	stats := whisper.Stats()
//	return Info{
//		Memory:         stats.memoryUsed,
//		Messages:       len(whisper.messageQueue),
//		GetMinPow:         whisper.GetMinPow(),
//		MaxMessageSize: whisper.MaxMessageSize(),
//	}
//}

//// SetMaxMessageSize sets the maximum message size that is accepted.
//// Upper limit is defined by MaxMessageSize.
//func (whisper *Whisper) SetMaxMessageSize(size uint32) (bool, error) {
//	return true, whisper.SetMaxMessageSize(size)
//}
//
//// SetMinPoW sets the minimum PoW, and notifies the peers.
//func (whisper *Whisper) SetMinPoW(pow float64) (bool, error) {
//	return true, whisper.SetMinPoW(pow)
//}
//
//// SetBloomFilter sets the new value of bloom filter, and notifies the peers.
//func (whisper *Whisper) SetBloomFilter(bloom []byte) (bool, error) {
//	return true, whisper.SetBloomFilter(bloom)
//}

//// MarkTrustedPeer marks a peer trusted, which will allow it to send historic (expired) messages.
//// Note: This function is not adding new nodes, the node needs to exists as a peer.
//func (whisper *Whisper) MarkTrustedPeer(url string) (bool, error) {
//	n, err := enode.Parse(enode.ValidSchemes, url)
//	if err != nil {
//		return false, err
//	}
//	return true, whisper.AllowP2PMessagesFromPeer(n.ID().Bytes())
//}

// NewKeyPair generates a new public and private key pair for message decryption and encryption.
// It returns an ID that can be used to refer to the keypair.
//func (whisper *Whisper) NewKeyPair() (string, error) {
//	return whisper.NewKeyPair()
//}

// AddPrivateKey imports the given private key.
func (whisper *Whisper) AddPrivateKey(privateKey []byte) (string, error) {
	key, err := crypto.ToECDSA(privateKey)
	if err != nil {
		return "", err
	}
	return whisper.AddKeyPair(key)
}

// DeleteKey removes the key with the given key if it exists.
//func (whisper *Whisper) DeleteKey(key string) (bool, error) {
//	if ok := whisper.DeleteKey(key); ok {
//		return true, nil
//	}
//	return false, fmt.Errorf("key pair %s not found", key)
//}

// HasKey returns an indication if the node has a key pair that is associated with the given id.
//func (whisper *Whisper) HasKey(id string) bool {
//	return whisper.HasKey(id)
//}

// GetPublicKey returns the public key associated with the given key. The key is the hex
// encoded representation of a key in the form specified in section 4.3.6 of ANSI X9.62.
func (whisper *Whisper) GetPublicKey(id string) ([]byte, error) {
	key, err := whisper.GetPrivateKey(id)
	if err != nil {
		return []byte{}, err
	}
	return crypto.FromECDSAPub(&key.PublicKey), nil
}

// GetPrivateKey returns the private key associated with the given key. The key is the hex
// encoded representation of a key in the form specified in section 4.3.6 of ANSI X9.62.
func (whisper *Whisper) GetPrivateKeyAPI(id string) ([]byte, error) {
	key, err := whisper.GetPrivateKey(id)
	if err != nil {
		return []byte{}, err
	}
	return crypto.FromECDSA(key), nil
}

// NewSymKey generate a random symmetric key.
// It returns an ID that can be used to refer to the key.
// Can be used encrypting and decrypting messages where the key is known to both parties.
//func (whisper *Whisper) NewSymKey() (string, error) {
//	return whisper.GenerateSymKey()
//}

// AddSymKey import a symmetric key.
// It returns an ID that can be used to refer to the key.
// Can be used encrypting and decrypting messages where the key is known to both parties.
func (whisper *Whisper) AddSymKey(key string) (string, error) {
	val, _ := hex.DecodeString(key)
	return whisper.AddSymKeyDirect(val)
}

//// GenerateSymKeyFromPassword derive a key from the given password, stores it, and returns its ID.
//func (whisper *Whisper) GenerateSymKeyFromPassword(passwd string) (string, error) {
//	return whisper.AddSymKeyFromPassword(passwd)
//}

//// HasSymKey returns an indication if the node has a symmetric key associated with the given key.
//func (whisper *Whisper) HasSymKey(id string) bool {
//	return whisper.HasSymKey(id)
//}

//// GetSymKey returns the symmetric key associated with the given id.
//func (whisper *Whisper) GetSymKey(id string) ([]byte, error) {
//	return whisper.GetSymKey(id)
//}

// DeleteSymKey deletes the symmetric key that is associated with the given id.
//func (whisper *Whisper) DeleteSymKey(id string) bool {
//	return whisper.DeleteSymKey(id)
//}

// NewMessage represents a new whisper message that is posted through the RPC.
type NewMessage struct {
	SymKeyID   string
	PublicKey  []byte
	Sig        string
	TTL        uint32
	Topic      Topic
	Payload    []byte
	Padding    []byte
	PowTime    uint32
	PowTarget  float64
	TargetPeer string
}

//type newMessageOverride struct {
//	PublicKey []byte
//	Payload   []byte
//	Padding   []byte
//}

// Post posts a message on the Whisper network.
// returns the hash of the message in case of success.
func (whisper *Whisper) Post(req NewMessage) ([]byte, error) {
	var (
		symKeyGiven = len(req.SymKeyID) > 0
		pubKeyGiven = len(req.PublicKey) > 0
		err         error
	)

	// user must specify either a symmetric or an asymmetric key
	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
		return nil, ErrSymAsym
	}

	params := &MessageParams{
		TTL:      req.TTL,
		Payload:  req.Payload,
		Padding:  req.Padding,
		WorkTime: req.PowTime,
		PoW:      req.PowTarget,
		Topic:    req.Topic,
	}

	// Set key that is used to sign the message
	if len(req.Sig) > 0 {
		if params.Src, err = whisper.GetPrivateKey(req.Sig); err != nil {
			return nil, err
		}
	}

	// Set symmetric key that is used to encrypt the message
	if symKeyGiven {
		if params.Topic == (Topic{}) { // topics are mandatory with symmetric encryption
			return nil, ErrNoTopics
		}
		if params.KeySym, err = whisper.GetSymKey(req.SymKeyID); err != nil {
			return nil, err
		}
		if !validateDataIntegrity(params.KeySym, aesKeyLength) {
			return nil, ErrInvalidSymmetricKey
		}
	}

	// Set asymmetric key that is used to encrypt the message
	if pubKeyGiven {
		if params.Dst, err = crypto.UnmarshalPubkey(req.PublicKey); err != nil {
			return nil, ErrInvalidPublicKey
		}
	}

	// encrypt and sent message
	whisperMsg, err := NewSentMessage(params)
	if err != nil {
		return nil, err
	}

	var result []byte
	env, err := whisperMsg.Wrap(params)
	if err != nil {
		return nil, err
	}

	// ensure that the message PoW meets the node's minimum accepted PoW
	if req.PowTarget < whisper.GetMinPow() {
		return nil, ErrTooLowPoW
	}

	err = whisper.Send(env)
	if err == nil {
		hash := env.Hash()
		result = hash[:]
	}
	return result, err
}

// Criteria holds various filter options for inbound messages.
type Criteria struct {
	SymKeyID     string
	PrivateKeyID string
	Sig          []byte
	MinPow       float64
	Topics       []Topic
	AllowP2P     bool
}

//type criteriaOverride struct {
//	Sig []byte
//}

// Messages set up a subscription that fires events when messages arrive that match
// the given set of criteria.
//func (whisper *Whisper) Messages(crit Criteria) (*rpc.Subscription, error) {
//	var (
//		symKeyGiven = len(crit.SymKeyID) > 0
//		pubKeyGiven = len(crit.PrivateKeyID) > 0
//		err         error
//	)
//
//	// ensure that the RPC connection supports subscriptions
//	notifier, supported := rpc.NotifierFromContext(ctx)
//	if !supported {
//		return nil, rpc.ErrNotificationsUnsupported
//	}
//
//	// user must specify either a symmetric or an asymmetric key
//	if (symKeyGiven && pubKeyGiven) || (!symKeyGiven && !pubKeyGiven) {
//		return nil, ErrSymAsym
//	}
//
//	filter := Filter{
//		PoW:      crit.GetMinPow,
//		Messages: make(map[[32]byte]*ReceivedMessage),
//		AllowP2P: crit.AllowP2P,
//	}
//
//	if len(crit.Sig) > 0 {
//		if filter.Src, err = crypto.UnmarshalPubkey(crit.Sig); err != nil {
//			return nil, ErrInvalidSigningPubKey
//		}
//	}
//
//	for i, bt := range crit.Topics {
//		if len(bt) == 0 || len(bt) > 4 {
//			return nil, fmt.Errorf("subscribe: topic %d has wrong size: %d", i, len(bt))
//		}
//		filter.Topics = append(filter.Topics, bt[:])
//	}
//
//	// listen for message that are encrypted with the given symmetric key
//	if symKeyGiven {
//		if len(filter.Topics) == 0 {
//			return nil, ErrNoTopics
//		}
//		key, err := whisper.GetSymKey(crit.SymKeyID)
//		if err != nil {
//			return nil, err
//		}
//		if !validateDataIntegrity(key, aesKeyLength) {
//			return nil, ErrInvalidSymmetricKey
//		}
//		filter.KeySym = key
//		filter.SymKeyHash = crypto.Keccak256Hash(filter.KeySym)
//	}
//
//	// listen for messages that are encrypted with the given public key
//	if pubKeyGiven {
//		filter.KeyAsym, err = whisper.GetPrivateKey(crit.PrivateKeyID)
//		if err != nil || filter.KeyAsym == nil {
//			return nil, ErrInvalidPublicKey
//		}
//	}
//
//	id, err := whisper.Subscribe(&filter)
//	if err != nil {
//		return nil, err
//	}
//
//	// create subscription and start waiting for message events
//	rpcSub := notifier.CreateSubscription()
//	go func() {
//		// for now poll internally, refactor whisper internal for channel support
//		ticker := time.NewTicker(250 * time.Millisecond)
//		defer ticker.Stop()
//
//		for {
//			select {
//			case <-ticker.C:
//				if filter := whisper.GetFilter(id); filter != nil {
//					for _, rpcMessage := range toMessage(filter.Retrieve()) {
//						if err := notifier.Notify(rpcSub.ID, rpcMessage); err != nil {
//							fmt.Println("Failed to send notification", "err", err)
//						}
//					}
//				}
//			case <-rpcSub.Err():
//				whisper.Unsubscribe(id)
//				return
//			case <-notifier.Closed():
//				whisper.Unsubscribe(id)
//				return
//			}
//		}
//	}()
//
//	return rpcSub, nil
//}

// Message is the RPC representation of a whisper message.
type Message struct {
	Sig       []byte
	TTL       uint32
	Timestamp uint32
	Topic     Topic
	Payload   []byte
	Padding   []byte
	PoW       float64
	Hash      []byte
	Dst       []byte
}

//type messageOverride struct {
//	Sig     []byte
//	Payload []byte
//	Padding []byte
//	Hash    []byte
//	Dst     []byte
//}

// ToWhisperMessage converts an internal message into an API version.
func ToWhisperMessage(message *ReceivedMessage) *Message {
	msg := Message{
		Payload:   message.Payload,
		Padding:   message.Padding,
		Timestamp: message.Sent,
		TTL:       message.TTL,
		PoW:       message.PoW,
		Hash:      message.EnvelopeHash[:],
		Topic:     message.Topic,
	}

	if message.Dst != nil {
		b := crypto.FromECDSAPub(message.Dst)
		if b != nil {
			msg.Dst = b
		}
	}

	if isMessageSigned(message.Raw[0]) {
		b := crypto.FromECDSAPub(message.SigToPubKey())
		if b != nil {
			msg.Sig = b
		}
	}

	return &msg
}

// toMessage converts a set of messages to its RPC representation.
func toMessage(messages []*ReceivedMessage) []*Message {
	msgs := make([]*Message, len(messages))
	for i, msg := range messages {
		msgs[i] = ToWhisperMessage(msg)
	}
	return msgs
}

// GetFilterMessages returns the messages that match the filter criteria and
// are received between the last poll and now.
func (whisper *Whisper) GetFilterMessages(id string) ([]*Message, error) {
	f := whisper.GetFilter(id)
	if f == nil {
		return nil, fmt.Errorf("filter not found")
	}

	receivedMessages := f.Retrieve()
	messages := make([]*Message, 0, len(receivedMessages))
	for _, msg := range receivedMessages {
		messages = append(messages, ToWhisperMessage(msg))
	}

	return messages, nil
}

// DeleteMessageFilter deletes a filter.
//func (whisper *Whisper) DeleteMessageFilter(id string) (bool, error) {
//	return true, whisper.Unsubscribe(id)
//}

// NewMessageFilter creates a new filter that can be used to poll for
// (new) messages that satisfy the given criteria.
func (whisper *Whisper) NewMessageFilter(req Criteria) (string, error) {
	var (
		src     *ecdsa.PublicKey
		keySym  []byte
		keyAsym *ecdsa.PrivateKey
		topics  [][]byte

		symKeyGiven  = len(req.SymKeyID) > 0
		asymKeyGiven = len(req.PrivateKeyID) > 0

		err error
	)

	// user must specify either a symmetric or an asymmetric key
	if (symKeyGiven && asymKeyGiven) || (!symKeyGiven && !asymKeyGiven) {
		return "", ErrSymAsym
	}

	if len(req.Sig) > 0 {
		if src, err = crypto.UnmarshalPubkey(req.Sig); err != nil {
			return "", ErrInvalidSigningPubKey
		}
	}

	if symKeyGiven {
		if keySym, err = whisper.GetSymKey(req.SymKeyID); err != nil {
			return "", err
		}
		if !validateDataIntegrity(keySym, aesKeyLength) {
			return "", ErrInvalidSymmetricKey
		}
	}

	if asymKeyGiven {
		if keyAsym, err = whisper.GetPrivateKey(req.PrivateKeyID); err != nil {
			return "", err
		}
	}

	if len(req.Topics) > 0 {
		topics = make([][]byte, len(req.Topics))
		for i, topic := range req.Topics {
			topics[i] = make([]byte, TopicLength)
			copy(topics[i], topic[:])
		}
	}

	f := &Filter{
		Src:      src,
		KeySym:   keySym,
		KeyAsym:  keyAsym,
		PoW:      req.MinPow,
		AllowP2P: req.AllowP2P,
		Topics:   topics,
		Messages: make(map[[32]byte]*ReceivedMessage),
	}

	s, err := whisper.filters.AddFilter(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}
	return s, err
}
