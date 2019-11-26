package gossiper

import "sync"

// MutexStatus struct
type MutexStatus struct {
	Status map[string]uint32
	Mutex  sync.RWMutex
}

func (gossiper *Gossiper) getStatusToSend() *GossipPacket {

	gossiper.myRumorStatus.Mutex.RLock()
	defer gossiper.myRumorStatus.Mutex.RUnlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range gossiper.myRumorStatus.Status {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return &GossipPacket{Status: statusPacket}
}

func (gossiper *Gossiper) getPeerStatusForPeer(otherStatus []PeerStatus) *PeerStatus {

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	gossiper.myRumorStatus.Mutex.RLock()
	defer gossiper.myRumorStatus.Mutex.RUnlock()

	for origin, nextID := range gossiper.myRumorStatus.Status {
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown {
			return &PeerStatus{Identifier: origin, NextID: 1}
		} else if nextID > id {
			return &PeerStatus{Identifier: origin, NextID: id}
		}
	}

	return nil
}

func (gossiper *Gossiper) isPeerStatusNeeded(otherStatus []PeerStatus) bool {

	gossiper.myRumorStatus.Mutex.RLock()
	defer gossiper.myRumorStatus.Mutex.RUnlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := gossiper.myRumorStatus.Status[elem.Identifier]
		if !isOriginKnown {
			return true
		} else if elem.NextID > id {
			return true
		}
	}

	return false
}
