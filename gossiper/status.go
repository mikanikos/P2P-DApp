package gossiper

import "sync"

// MutexStatus struct
type MutexStatus struct {
	Status map[string]uint32
	Mutex  sync.Mutex
}

func (gossiper *Gossiper) getStatusToSend() *GossipPacket {

	// gossiper.myStatus.Mutex.Lock()
	// defer gossiper.myStatus.Mutex.Unlock()

	// myStatus := make([]PeerStatus, 0)

	// for origin, nextID := range gossiper.myStatus.Status {
	// 	myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	// }

	// statusPacket := &StatusPacket{Want: myStatus}
	// return &GossipPacket{Status: statusPacket}

	myStatus := make([]PeerStatus, 0)

	gossiper.myStatus.Range(func(key interface{}, value interface{}) bool {
		myStatus = append(myStatus, PeerStatus{Identifier: key.(string), NextID: value.(uint32)})
		return true
	})

	statusPacket := &StatusPacket{Want: myStatus}
	return &GossipPacket{Status: statusPacket}
}

func (gossiper *Gossiper) getPeerStatusOtherNeeds(otherStatus []PeerStatus) *PeerStatus {

	// originIDMap := make(map[string]uint32)
	// for _, elem := range otherStatus {
	// 	originIDMap[elem.Identifier] = elem.NextID
	// }

	// for origin, nextID := range gossiper.myStatus.Status {
	// 	id, isOriginKnown := originIDMap[origin]
	// 	if !isOriginKnown {
	// 		return &PeerStatus{Identifier: origin, NextID: 1}
	// 	} else if nextID > id {
	// 		return &PeerStatus{Identifier: origin, NextID: id}
	// 	}
	// }

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	var peerStatus *PeerStatus

	gossiper.myStatus.Range(func(key interface{}, value interface{}) bool {
		origin := key.(string)
		nextID := value.(uint32)
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown {
			peerStatus = &PeerStatus{Identifier: origin, NextID: 1}
			return false
		} else if nextID > id {
			peerStatus = &PeerStatus{Identifier: origin, NextID: id}
			return false
		}
		return true
	})

	return peerStatus
}

func (gossiper *Gossiper) doINeedSomething(otherStatus []PeerStatus) bool {

	// gossiper.myStatus.Mutex.Lock()
	// defer gossiper.myStatus.Mutex.Unlock()

	// for _, elem := range otherStatus {
	// 	id, isOriginKnown := gossiper.myStatus.Status[elem.Identifier]
	// 	if !isOriginKnown {
	// 		return true
	// 	} else if elem.NextID > id {
	// 		return true
	// 	}
	// }

	for _, elem := range otherStatus {
		value, isOriginKnown := gossiper.myStatus.Load(elem.Identifier)
		if isOriginKnown {
			id := value.(uint32)
			if elem.NextID > id {
				return true
			}
		} else {
			return true
		}
	}

	return false
}
