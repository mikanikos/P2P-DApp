package gossiper

import (
	"sync"
)

// MutexStatus struct
type MutexStatus struct {
	Status map[string]uint32
	Mutex  sync.RWMutex
}

func getStatusToSend(status *MutexStatus) *StatusPacket {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range status.Status {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return statusPacket
}

func getPeerStatusForPeer(otherStatus []PeerStatus, status *MutexStatus) *PeerStatus {

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for origin, nextID := range status.Status {
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown && nextID != 0 {
			return &PeerStatus{Identifier: origin, NextID: 1}
		} else if nextID > id {
			return &PeerStatus{Identifier: origin, NextID: id}
		}
	}
	return nil
}

func isPeerStatusNeeded(otherStatus []PeerStatus, status *MutexStatus) bool {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := status.Status[elem.Identifier]
		if !isOriginKnown && elem.NextID != 0 {
			return true
		} else if elem.NextID > id {
			return true
		}
	}
	return false
}
