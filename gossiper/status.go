package gossiper

import "sync"

// MutexStatus struct
type MutexStatus struct {
	Status map[string]uint32
	Mutex  sync.RWMutex
}

func (gossiper *Gossiper) getStatusToSend() *GossipPacket {

	gossiper.myStatus.Mutex.RLock()
	defer gossiper.myStatus.Mutex.RUnlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range gossiper.myStatus.Status {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return &GossipPacket{Status: statusPacket}
}

func (gossiper *Gossiper) getPeerStatusOtherNeeds(otherStatus []PeerStatus) *PeerStatus {

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	gossiper.myStatus.Mutex.RLock()
	defer gossiper.myStatus.Mutex.RUnlock()

	for origin, nextID := range gossiper.myStatus.Status {
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown {
			return &PeerStatus{Identifier: origin, NextID: 1}
		} else if nextID > id {
			return &PeerStatus{Identifier: origin, NextID: id}
		}
	}

	return nil
}

func (gossiper *Gossiper) doINeedSomething(otherStatus []PeerStatus) bool {

	gossiper.myStatus.Mutex.RLock()
	defer gossiper.myStatus.Mutex.RUnlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := gossiper.myStatus.Status[elem.Identifier]
		if !isOriginKnown {
			return true
		} else if elem.NextID > id {
			return true
		}
	}

	return false
}
