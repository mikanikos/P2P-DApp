package gossiper

import "sync"

// MutexSequenceID struct
type MutexSequenceID struct {
	ID    uint32
	Mutex sync.Mutex
}

func (gossiper *Gossiper) incerementID() {
	mID := &gossiper.seqID
	mID.Mutex.Lock()
	defer mID.Mutex.Unlock()
	mID.ID = mID.ID + 1
}

func (gossiper *Gossiper) getIDAtomic() uint32 {
	mID := &gossiper.seqID
	mID.Mutex.Lock()
	defer mID.Mutex.Unlock()
	idCopy := mID.ID
	return idCopy
}
