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
	Mutex sync.Mutex
}

// AddPeer to peers list
func (gossiper *Gossiper) AddPeer(peer *net.UDPAddr) {
	peers := &gossiper.peers
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	contains := false
	for _, p := range peers.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		peers.Peers = append(peers.Peers, peer)
	}
}

func (gossiper *Gossiper) printPeers() {
	peers := gossiper.GetPeersAtomic()
	listPeers := helpers.GetArrayStringFromAddresses(peers)
	fmt.Println("PEERS " + strings.Join(listPeers, ","))
}

// GetPeersAtomic in concurrent environment
func (gossiper *Gossiper) GetPeersAtomic() []*net.UDPAddr {
	peers := &gossiper.peers
	peers.Mutex.Lock()
	defer peers.Mutex.Unlock()
	peerCopy := make([]*net.UDPAddr, len(peers.Peers))
	copy(peerCopy, peers.Peers)
	return peerCopy
}
