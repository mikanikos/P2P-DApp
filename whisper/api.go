package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
)

// NewMessage contain all the fields to create a whisper message
type NewMessage struct {
	SymKeyID  string
	PublicKey []byte
	TTL       uint32
	Topic     Topic
	Pow       float64
	Payload   []byte
}

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
//func (whisper *Whisper) Messages(crit Criteria) (chan *ReceivedMessage, error) {
//	isSymKey := len(crit.SymKeyID) > 0
//	isPrivKey := len(crit.PrivateKeyID) > 0
//
//	// either symmetric or asymmetric key
//	if (isSymKey && isPrivKey) || (!isSymKey && !isPrivKey) {
//		return nil, fmt.Errorf("either private or symmetric key")
//	}
//
//
//	filter := Filter{
//		Messages: make(map[[32]byte]*ReceivedMessage),
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
//	if isSymKey {
//		if len(filter.Topics) == 0 {
//			return nil, fmt.Errorf("no topics provided")
//		}
//		key, err := whisper.GetSymKeyFromID(crit.SymKeyID)
//		if err != nil {
//			return nil, err
//		}
//		if len(key) != aesKeyLength {
//			return nil, fmt.Errorf("invalid key length")
//		}
//		filter.KeySym = key
//	}
//
//	// listen for messages that are encrypted with the given public key
//	if isPrivKey {
//		key, err := whisper.GetPrivateKey(crit.PrivateKeyID)
//		if err != nil || key == nil {
//			return nil, fmt.Errorf("invalid key")
//		}
//		filter.KeyAsym = key
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
	f := whisper.filters.GetFilterFromID(id)
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
