package gossiper

import (
	"sync"
)

// MutexOrigins struct
type MutexOrigins struct {
	Origins []string
	Mutex   sync.RWMutex
}

// AddOrigin to origins list
func (gossiper *Gossiper) addOrigin(origin string) {
	gossiper.origins.Mutex.Lock()
	defer gossiper.origins.Mutex.Unlock()
	contains := false
	for _, o := range gossiper.origins.Origins {
		if o == origin {
			contains = true
			break
		}
	}
	if !contains {
		gossiper.origins.Origins = append(gossiper.origins.Origins, origin)
	}
}

// GetOriginsAtomic in concurrent environment
func (gossiper *Gossiper) GetOriginsAtomic() []string {
	gossiper.origins.Mutex.RLock()
	defer gossiper.origins.Mutex.RUnlock()
	originsCopy := make([]string, len(gossiper.origins.Origins))
	copy(originsCopy, gossiper.origins.Origins)
	return originsCopy
}
