package gossiper

import "sync"

// MutexStatus struct
type MutexStatus struct {
	Status map[string]uint32
	Mutex  sync.Mutex
}

func (gossiper *Gossiper) getStatusToSend() *GossipPacket {

	gossiper.myStatus.Mutex.Lock()
	defer gossiper.myStatus.Mutex.Unlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range gossiper.myStatus.Status {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return &GossipPacket{Status: statusPacket}
}

func (gossiper *Gossiper) getPeerStatusOtherNeeds(otherStatus []PeerStatus) *PeerStatus {

	gossiper.myStatus.Mutex.Lock()
	defer gossiper.myStatus.Mutex.Unlock()

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

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

func (gossiper *Gossiper) getPeerStatusINeed(otherStatus []PeerStatus) *PeerStatus {

	gossiper.myStatus.Mutex.Lock()
	defer gossiper.myStatus.Mutex.Unlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := gossiper.myStatus.Status[elem.Identifier]
		if !isOriginKnown {
			return &PeerStatus{Identifier: elem.Identifier, NextID: 1}
		} else if elem.NextID > id {
			return &PeerStatus{Identifier: elem.Identifier, NextID: id}
		}
	}

	return nil
}
