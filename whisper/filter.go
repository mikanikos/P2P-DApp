package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
	"sync"
)

// Filter is a message filter
type Filter struct {
	Src     *ecies.PublicKey
	KeyAsym *ecies.PrivateKey
	KeySym  []byte

	Topics [][]byte

	Messages map[[32]byte]*ReceivedMessage
	Mutex    sync.RWMutex
}

// FilterStorage has all the filters
type FilterStorage struct {
	subscribers map[string]*Filter

	topicToFilters map[Topic]map[*Filter]struct{}
	//allTopicsMatcher map[*Filter]struct{}

	//whisper *Whisper
	mutex sync.RWMutex
}

// NewFilterStorage returns a newly created filter collection
func NewFilterStorage() *FilterStorage {
	return &FilterStorage{
		subscribers:    make(map[string]*Filter),
		topicToFilters: make(map[Topic]map[*Filter]struct{}),
		//allTopicsMatcher: make(map[*Filter]struct{}),
		//whisper:          w,
	}
}

// AddFilter will handleEnvelope a new filter to the filter collection
func (fs *FilterStorage) AddFilter(filter *Filter) (string, error) {
	if filter.KeySym != nil && filter.KeyAsym != nil {
		return "", fmt.Errorf("filters must choose between symmetric and asymmetric keys")
	}

	if filter.Messages == nil {
		filter.Messages = make(map[[32]byte]*ReceivedMessage)
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", err
	}

	fs.mutex.Lock()

	_, loaded := fs.subscribers[id]
	if loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	fs.subscribers[id] = filter
	for _, t := range filter.Topics {
		topic := ConvertBytesToTopic(t)
		if fs.topicToFilters[topic] == nil {
			fs.topicToFilters[topic] = make(map[*Filter]struct{})
		}
		fs.topicToFilters[topic][filter] = struct{}{}
	}

	fs.mutex.Unlock()

	return id, err
}

// RemoveFilter will remove a filter from id given
func (fs *FilterStorage) RemoveFilter(id string) bool {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	sub, loaded := fs.subscribers[id]
	if loaded {
		delete(fs.subscribers, id)
		//delete(fs.allTopicsMatcher, sub)
		for _, topic := range sub.Topics {
			delete(fs.topicToFilters[ConvertBytesToTopic(topic)], sub)
		}
		return true
	}
	return false
}

//// addTopicMatcher adds a filter to the topic matchers.
//// If the filter's Topics array is empty, it will be tried on every topic.
//// Otherwise, it will be tried on the topics specified.
//func (fs *FilterStorage) addTopicMatcher(subscriber *Filter) {
//	if len(subscriber.Topics) == 0 {
//		fs.allTopicsMatcher[subscriber] = struct{}{}
//	} else {
//
//	}
//}

// getSubscribersByTopic returns all filters given a topic
func (fs *FilterStorage) getSubscribersByTopic(topic Topic) []*Filter {
	res := make([]*Filter, 0, len(fs.topicToFilters[topic]))
	for subscriber := range fs.topicToFilters[topic] {
		res = append(res, subscriber)
	}
	return res
}

// GetFilterFromID returns a filter from the collection with a specific ID
func (fs *FilterStorage) GetFilterFromID(id string) *Filter {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.subscribers[id]
}

// NotifySubscribers notifies filters of matching envelope topic
func (fs *FilterStorage) NotifySubscribers(env *Envelope) {
	var msg *ReceivedMessage

	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	candidates := fs.getSubscribersByTopic(env.Topic)
	for _, sub := range candidates {
		msg = env.GetMessageFromEnvelope(sub)
		if msg == nil {
			fmt.Println("failed to open message")
		} else {
			if sub.Src == nil || msg.Src.Equals(sub.Src) {
				sub.Mutex.Lock()
				if _, exist := sub.Messages[msg.EnvelopeHash]; !exist {
					sub.Messages[msg.EnvelopeHash] = msg
				}
				sub.Mutex.Unlock()
			}
		}
	}
}

func (f *Filter) isAsymmetricEncryption() bool {
	return f.KeyAsym != nil
}

func (f *Filter) isSymmetricEncryption() bool {
	return f.KeySym != nil
}

// GetReceivedMessagesFromFilter returns received messages given for a filter
func (f *Filter) GetReceivedMessagesFromFilter() (messages []*ReceivedMessage) {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	messages = make([]*ReceivedMessage, 0, len(f.Messages))
	for _, msg := range f.Messages {
		messages = append(messages, msg)
	}

	f.Messages = make(map[[32]byte]*ReceivedMessage)
	return messages
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
//	if f.isAsymmetricEncryption() && msg.isAsymmetricEncryption() {
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
