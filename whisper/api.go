package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
)

// List of errors
//var (
//	ErrSymAsym              = fmt.Errorf("specify either a symmetric or an asymmetric key")
//	ErrInvalidSymmetricKey  = fmt.Errorf("invalid symmetric key")
//	ErrInvalidPublicKey     = fmt.Errorf("invalid public key")
//	ErrInvalidSigningPubKey = fmt.Errorf("invalid signing public key")
//	ErrTooLowPoW            = fmt.Errorf("message rejected, PoW too low")
//	ErrNoTopics             = fmt.Errorf("missing topic(s)")
//)

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

//// MarkTrustedPeer marks a address trusted, which will allow it to send historic (expired) messages.
//// Note: This function is not adding new nodes, the node needs to exists as a address.
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

// NewSymKey generate a random symmetric key.
// It returns an ID that can be used to refer to the key.
// Can be used encrypting and decrypting messages where the key is known to both parties.
//func (whisper *Whisper) NewSymKey() (string, error) {
//	return whisper.GenerateSymKey()
//}

//// GenerateSymKeyFromPassword derive a key from the given password, stores it, and returns its ID.
//func (whisper *Whisper) GenerateSymKeyFromPassword(passwd string) (string, error) {
//	return whisper.AddSymKeyFromPassword(passwd)
//}

//// HasSymKey returns an indication if the node has a symmetric key associated with the given key.
//func (whisper *Whisper) HasSymKey(id string) bool {
//	return whisper.HasSymKey(id)
//}

//// GetSymKeyFromID returns the symmetric key associated with the given id.
//func (whisper *Whisper) GetSymKeyFromID(id string) ([]byte, error) {
//	return whisper.GetSymKeyFromID(id)
//}

// DeleteSymKey deletes the symmetric key that is associated with the given id.
//func (whisper *Whisper) DeleteSymKey(id string) bool {
//	return whisper.DeleteSymKey(id)
//}

// NewMessage contain all the fields to create a whisper message
type NewMessage struct {
	SymKeyID  string
	PublicKey []byte
	TTL       uint32
	Topic     Topic
	Pow       float64
	Payload   []byte
}

//type newMessageOverride struct {
//	PublicKey []byte
//	Payload   []byte
//	Padding   []byte
//}

// NewWhisperMessage posts a message on the Whisper network.
// returns the hash of the message in case of success.
func (whisper *Whisper) NewWhisperMessage(message NewMessage) ([]byte, error) {

	isSymKey := len(message.SymKeyID) > 0
	isPubKey := len(message.PublicKey) > 0

	// either symmetric or asymmetric key
	if (isSymKey && isPubKey) || (!isSymKey && !isPubKey) {
		return nil, fmt.Errorf("specifiy either public or symmetric key")
	}

	// check pow
	if message.Pow < whisper.GetMinPow() {
		return nil, fmt.Errorf("low pow")
	}

	params := &MessageParams{
		TTL:     message.TTL,
		Payload: message.Payload,
		PoW:     message.Pow,
		Topic:   message.Topic,
	}

	//// Set key that is used to sign the message
	//if len(message.Sig) > 0 {
	//	if params.Src, err = whisper.GetPrivateKey(message.Sig); err != nil {
	//		return nil, err
	//	}
	//}

	// check crypt keys
	if isSymKey {
		if params.Topic == (Topic{}) {
			return nil, fmt.Errorf("need topic with symmetric key")
		}
		key, err := whisper.GetSymKeyFromID(message.SymKeyID)
		if err != nil {
			return nil, err
		}
		params.KeySym = key

		if len(params.KeySym) != aesKeyLength {
			return nil, fmt.Errorf("invalid key length")
		}
	}

	if isPubKey {
		key, err := ecies.NewPublicKeyFromBytes(message.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key")
		}
		params.Dst = key
	}

	// encrypt message
	//whisperMsg, err := NewSentMessage(params)
	//if err != nil {
	//	return nil, err
	//}

	// encrypt and create envelope
	var result []byte
	env, err := params.GetEnvelopeFromMessage()
	if err != nil {
		return nil, err
	}

	err = whisper.Send(env)
	if err == nil {
		hash := env.Hash()
		result = hash[:]
	}
	return result, err
}

// Criteria holds various filter options for messages
type Criteria struct {
	SymKeyID     string
	PrivateKeyID string
	MinPow       float64
	Topics       []Topic
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
//		key, err := whisper.GetSymKeyFromID(crit.SymKeyID)
//		if err != nil {
//			return nil, err
//		}
//		if !validateDataIntegrity(key, aesKeyLength) {
//			return nil, ErrInvalidSymmetricKey
//		}
//		filter.KeySym = key
//		filter.SymKeyHash = crypto.sha3.Sum256(filter.KeySym)
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
//					for _, rpcMessage := range toMessage(filter.GetReceivedMessagesFromFilter()) {
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

// ReceivedMessage is the RPC representation of a whisper message.
//type ReceivedMessage struct {
//	Sig       []byte
//	TTL       uint32
//	Timestamp uint32
//	Topic     Topic
//	Payload   []byte
//	Dst       []byte
//}

//type messageOverride struct {
//	Sig     []byte
//	Payload []byte
//	Padding []byte
//	Hash    []byte
//	Dst     []byte
//}

// ToWhisperMessage converts an internal message into an API version.
//func ToWhisperMessage(message *ReceivedMessage) *ReceivedMessage {
//	msg := ReceivedMessage{
//		Payload:   message.Payload,
//		Timestamp: message.Sent,
//		TTL:       message.TTL,
//		Topic:     message.Topic,
//	}
//
//	if message.Dst != nil {
//		b := crypto.FromECDSAPub(message.Dst)
//		if b != nil {
//			msg.Dst = b
//		}
//	}
//
//	if isMessageSigned(message.Raw[0]) {
//		b := crypto.FromECDSAPub(message.SigToPubKey())
//		if b != nil {
//			msg.Sig = b
//		}
//	}
//
//	return &msg
//}

// toMessage converts a set of messages to its RPC representation.
//func toMessage(messages []*ReceivedMessage) []*ReceivedMessage {
//	msgs := make([]*ReceivedMessage, len(messages))
//	for i, msg := range messages {
//		msgs[i] = ToWhisperMessage(msg)
//	}
//	return msgs
//}

// GetFilterMessages returns the messages that match the filter criteria
func (whisper *Whisper) GetFilterMessages(id string) ([]*ReceivedMessage, error) {
	f := whisper.GetFilter(id)
	if f == nil {
		return nil, fmt.Errorf("filter not found")
	}

	receivedMessages := f.GetReceivedMessagesFromFilter()
	messages := make([]*ReceivedMessage, 0, len(receivedMessages))
	for _, msg := range receivedMessages {
		messages = append(messages, msg)
	}

	return messages, nil
}

// NewMessageFilter creates a new filter
func (whisper *Whisper) NewMessageFilter(req Criteria) (string, error) {
	var (
		src     *ecies.PublicKey
		keySym  []byte
		keyAsym *ecies.PrivateKey
		topics  [][]byte
		err     error
	)

	filter := &Filter{}

	isSymKey := len(req.SymKeyID) > 0
	isPrivKey := len(req.PrivateKeyID) > 0

	// either symmetric or asymmetric key
	if (isSymKey && isPrivKey) || (!isSymKey && !isPrivKey) {
		return "", fmt.Errorf("either private or symmetric key")
	}

	if isSymKey {
		key, err := whisper.GetSymKeyFromID(req.SymKeyID)
		if err != nil {
			return "", err
		}
		filter.KeySym = key
		if len(key) != aesKeyLength {
			return "", fmt.Errorf("invalid key length")
		}
	}

	if isPrivKey {
		key, err := whisper.GetPrivateKey(req.PrivateKeyID)
		if err != nil {
			return "", err
		}
		filter.KeyAsym = key
	}

	if len(req.Topics) > 0 {
		topics := make([][]byte, len(req.Topics))
		for i, topic := range req.Topics {
			topics[i] = make([]byte, TopicLength)
			copy(topics[i], topic[:])
		}
	}

	f := &Filter{
		Src:      src,
		KeySym:   keySym,
		KeyAsym:  keyAsym,
		Topics:   topics,
		Messages: make(map[[32]byte]*ReceivedMessage),
	}

	s, err := whisper.filters.AddFilter(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}
	return s, err
}
