package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
	"sync"
)

// Filter represents a Whisper message filter
type Filter struct {
	Src     *ecies.PublicKey  // Sender of the message
	KeyAsym *ecies.PrivateKey // Private Key of recipient
	KeySym  []byte            // Key associated with the Topic
	Topics  [][]byte          // Topics to filter messages with
	id      string            // unique identifier

	Messages map[[32]byte]*ReceivedMessage
	mutex    sync.RWMutex
}

// Filters represents a collection of filters
type Filters struct {
	subscribers map[string]*Filter

	topicMatcher     map[Topic]map[*Filter]struct{} // map a topic to the filters that are interested in being notified when a message matches that topic
	allTopicsMatcher map[*Filter]struct{}           // list all the filters that will be notified of a new message, no matter what its topic is

	whisper *Whisper
	mutex   sync.RWMutex
}

// NewFilters returns a newly created filter collection
func NewFilters(w *Whisper) *Filters {
	return &Filters{
		subscribers:         make(map[string]*Filter),
		topicMatcher:     make(map[Topic]map[*Filter]struct{}),
		allTopicsMatcher: make(map[*Filter]struct{}),
		whisper:          w,
	}
}

// AddFilter will add a new filter to the filter collection
func (fs *Filters) AddFilter(subscriber *Filter) (string, error) {
	if subscriber.KeySym != nil && subscriber.KeyAsym != nil {
		return "", fmt.Errorf("filters must choose between symmetric and asymmetric keys")
	}

	if subscriber.Messages == nil {
		subscriber.Messages = make(map[[32]byte]*ReceivedMessage)
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", err
	}

	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	if fs.subscribers[id] != nil {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	subscriber.id = id
	fs.subscribers[id] = subscriber
	fs.addTopicMatcher(subscriber)
	return id, err
}

// RemoveFilter will remove a filter whose id has been specified from
// the filter collection
func (fs *Filters) RemoveFilter(id string) bool {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	if fs.subscribers[id] != nil {
		fs.removeFromTopicMatchers(fs.subscribers[id])
		delete(fs.subscribers, id)
		return true
	}
	return false
}

// addTopicMatcher adds a filter to the topic matchers.
// If the filter's Topics array is empty, it will be tried on every topic.
// Otherwise, it will be tried on the topics specified.
func (fs *Filters) addTopicMatcher(subscriber *Filter) {
	if len(subscriber.Topics) == 0 {
		fs.allTopicsMatcher[subscriber] = struct{}{}
	} else {
		for _, t := range subscriber.Topics {
			topic := ConvertBytesToTopic(t)
			if fs.topicMatcher[topic] == nil {
				fs.topicMatcher[topic] = make(map[*Filter]struct{})
			}
			fs.topicMatcher[topic][subscriber] = struct{}{}
		}
	}
}

// removeFromTopicMatchers removes a filter from the topic matchers
func (fs *Filters) removeFromTopicMatchers(subscriber *Filter) {
	delete(fs.allTopicsMatcher, subscriber)
	for _, topic := range subscriber.Topics {
		delete(fs.topicMatcher[ConvertBytesToTopic(topic)], subscriber)
	}
}

// getsubscribersByTopic returns a slice containing the filters that
// match a specific topic
func (fs *Filters) getsubscribersByTopic(topic Topic) []*Filter {
	res := make([]*Filter, 0, len(fs.allTopicsMatcher))
	for subscriber := range fs.allTopicsMatcher {
		res = append(res, subscriber)
	}
	for subscriber := range fs.topicMatcher[topic] {
		res = append(res, subscriber)
	}
	return res
}

// GetFilter returns a filter from the collection with a specific ID
func (fs *Filters) GetFilter(id string) *Filter {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.subscribers[id]
}

// NotifySubscribers notifies any filter that has declared interest
// for the envelope's topic.
func (fs *Filters) NotifySubscribers(env *Envelope) {
	var msg *ReceivedMessage

	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	candidates := fs.getsubscribersByTopic(env.Topic)
	for _, subscriber := range candidates {
		msg = env.GetMessageFromEnvelope(subscriber)
		if msg == nil {
			fmt.Println("processing message: failed to open", "message", env.Hash(), "filter", subscriber.id)
		} else {
			fmt.Println("processing message: decrypted", "hash", env.Hash())
			if subscriber.Src == nil || msg.Src.Equals(subscriber.Src) {
				subscriber.Trigger(msg)
			}
		}
	}
}

func (f *Filter) isAsymmetricEncription() bool {
	return f.KeyAsym != nil
}

func (f *Filter) isSymmetricEncryption() bool {
	return f.KeySym != nil
}

// Trigger adds a yet-unknown message to the filter's list of
// received messages.
func (f *Filter) Trigger(msg *ReceivedMessage) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if _, exist := f.Messages[msg.EnvelopeHash]; !exist {
		f.Messages[msg.EnvelopeHash] = msg
	}
}

// Retrieve will return the list of all received messages associated
// to a filter.
func (f *Filter) Retrieve() (all []*ReceivedMessage) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	all = make([]*ReceivedMessage, 0, len(f.Messages))
	for _, msg := range f.Messages {
		all = append(all, msg)
	}

	f.Messages = make(map[[32]byte]*ReceivedMessage) // delete old messages
	return all
}

// MatchMessage checks if the filter matches an already decrypted
// message (i.e. a ReceivedMessage that has already been handled by
// MatchEnvelope when checked by a previous filter).
// Topics are not checked here, since this is done by topic matchers.
//func (f *Filter) MatchMessage(msg *ReceivedMessage) bool {
//	if f.PoW > 0 && msg.PoW < f.PoW {
//		return false
//	}
//
//	if f.isAsymmetricEncription() && msg.isAsymmetricEncryption() {
//		return IsPubKeyEqual(&f.KeyAsym.PublicKey, msg.Dst)
//	} else if f.isSymmetricEncryption() && msg.isSymmetricEncryption() {
//		return f.SymKeyHash == msg.SymKeyHash
//	}
//	return false
//}

// MatchEnvelope checks if it's worth decrypting the message. If
// it returns `true`, client code is expected to attempt decrypting
// the message and subsequently call MatchMessage.
// Topics are not checked here, since this is done by topic matchers.
//func (f *Filter) MatchEnvelope(envelope *Envelope) bool {
//	return f.PoW <= 0 || envelope.pow >= f.PoW
//}
