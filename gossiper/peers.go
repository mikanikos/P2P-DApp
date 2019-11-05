package gossiper

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/mikanikos/Peerster/helpers"
)

// MutexPeers struct
type MutexPeers struct {
	Peers []*net.UDPAddr
	Mutex sync.RWMutex
}

// AddPeer to peers list
func (gossiper *Gossiper) AddPeer(peer *net.UDPAddr) {
	gossiper.peers.Mutex.Lock()
	defer gossiper.peers.Mutex.Unlock()
	contains := false
	for _, p := range gossiper.peers.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		gossiper.peers.Peers = append(gossiper.peers.Peers, peer)
	}
}

func (gossiper *Gossiper) printPeers() {
	peers := gossiper.GetPeersAtomic()
	listPeers := helpers.GetArrayStringFromAddresses(peers)
	if hw1 {
		fmt.Println("PEERS " + strings.Join(listPeers, ","))
	}
}

// GetPeersAtomic in concurrent environment
func (gossiper *Gossiper) GetPeersAtomic() []*net.UDPAddr {
	gossiper.peers.Mutex.RLock()
	defer gossiper.peers.Mutex.RUnlock()
	peerCopy := make([]*net.UDPAddr, len(gossiper.peers.Peers))
	copy(peerCopy, gossiper.peers.Peers)
	return peerCopy
}
