package gossiper

import "sync"

// ConditionVariable struct
type ConditionVariable struct {
	Cond  sync.Cond
	Mutex sync.Mutex
}

func (gossiper *Gossiper) createOrGetMongerCond(peer string) *sync.Cond {

	_, exists := gossiper.mongerCond[peer]
	if !exists {
		m := sync.Mutex{}
		gossiper.mongerCond[peer] = sync.NewCond(&m)
	}
	return gossiper.mongerCond[peer]
}

func (gossiper *Gossiper) createOrGetSyncCond(peer string) *sync.Cond {

	_, exists := gossiper.syncCond[peer]
	if !exists {
		m := sync.Mutex{}
		gossiper.syncCond[peer] = sync.NewCond(&m)
	}
	return gossiper.syncCond[peer]
}
