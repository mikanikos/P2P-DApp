package whisper

import (
	"github.com/dedis/protobuf"
	"github.com/mikanikos/Peerster/gossiper"
	"math"
	"net"
	"sync"
	"time"
)

type Status struct {
	Pow   float64
	Bloom []byte
}

// RoutingHandler struct
type RoutingHandler struct {
	// peer -> aggregated bloom from that peer
	peerStatus map[string]*Status
	//bloomMutex sync.Mutex

	// peer -> min pow from that peer
	//peerMinPow map[string]float64
	//powMutex   sync.Mutex

	// track current last id (just an optimization in order to not iterate on the message storage every time)
	originLastID *gossiper.VectorClock
	mutex        sync.RWMutex
}

// NewRoutingHandler create new routing handler
func NewRoutingHandler() *RoutingHandler {
	return &RoutingHandler{
		peerStatus: make(map[string]*Status),
		//peerMinPow:   make(map[string]float64),
		originLastID: &gossiper.VectorClock{Entries: make(map[string]uint32)},
	}
}

//// StartRouteRumormongering with the specified timer
//func (whisper *Whisper) startRouteRumormongering() {
//
//	if routeRumorTimeout > 0 {
//
//		// create new rumor message
//		extPacket := gossiper.createRumorMessage("")
//
//		// broadcast it initially in order to start well
//		go gossiper.ConnectionHandler.BroadcastToPeers(extPacket, gossiper.GetPeers())
//
//		// start timer
//		timer := time.NewTicker(time.Duration(routeRumorTimeout) * time.Second)
//		for {
//			select {
//			// rumor monger rumor at each timeout
//			case <-timer.C:
//				// create new rumor message
//				extPacket := gossiper.createRumorMessage("")
//
//				// start rumormongering the message
//				go gossiper.startRumorMongering(extPacket, gossiper.Name, extPacket.Packet.Rumor.ID)
//			}
//		}
//	}
//}

// update routing table based on packet data
func (routingHandler *RoutingHandler) updateRoutingTable(whisperStatus *gossiper.WhisperStatus, address *net.UDPAddr) {

	routingHandler.mutex.Lock()
	defer routingHandler.mutex.Unlock()

	// if new packet with higher id, update table
	if routingHandler.updateLastOriginID(whisperStatus.Origin, whisperStatus.ID) {

		status, loaded := routingHandler.peerStatus[address.String()]
		if !loaded {
			routingHandler.peerStatus[address.String()] = &Status{}
			status, _ = routingHandler.peerStatus[address.String()]
		}

		if whisperStatus.Code == bloomFilterExCode || whisperStatus.Code == statusCode {
			if whisperStatus.Bloom != nil && len(whisperStatus.Bloom) == BloomFilterSize {
				if loaded {
					status.Bloom = AggregateBloom(whisperStatus.Bloom, status.Bloom)
				} else {
					status.Bloom = whisperStatus.Bloom
				}
			}
		}

		if whisperStatus.Code == powRequirementCode || whisperStatus.Code == statusCode {
			if !(math.IsInf(whisperStatus.Pow, 0) || math.IsNaN(whisperStatus.Pow) || whisperStatus.Pow < 0.0) {
				if loaded {
					status.Pow = math.Min(status.Pow, whisperStatus.Pow)
				} else {
					status.Pow = whisperStatus.Pow
				}
			}
		}
	}
}

//func (routingHandler *RoutingHandler) updatePowTable(origin string, idPacket uint32, address *net.UDPAddr, powReceived float64) {
//
//	routingHandler.mutex.Lock()
//	defer routingHandler.mutex.Unlock()
//
//	// if new packet with higher id, update table
//	if routingHandler.updateLastOriginID(origin, idPacket) {
//
//		powPeer, loaded := routingHandler.peerMinPow[address.String()]
//		if !loaded {
//			powPeer = powReceived
//		}
//		routingHandler.peerMinPow[address.String()] = math.Min(powPeer, powReceived)
//	}
//}

// check if packet is new (has higher id) from that source, in that case it updates the table
func (routingHandler *RoutingHandler) updateLastOriginID(origin string, id uint32) bool {
	isNew := false

	originMaxID, loaded := routingHandler.originLastID.Entries[origin]
	if !loaded {
		routingHandler.originLastID.Entries[origin] = id
		isNew = true
	} else {
		if id > originMaxID {
			routingHandler.originLastID.Entries[origin] = id
			isNew = true
		}
	}

	return isNew
}

// forward private message
func (whisper *Whisper) forwardEnvelope(envelope *Envelope) {

	packetToSend, _ := protobuf.Encode(envelope)
	packet := &gossiper.GossipPacket{WhisperPacket: &gossiper.WhisperPacket{Code: messagesCode, Payload: packetToSend, Size: uint32(len(packetToSend))}}

	whisper.routingHandler.mutex.RLock()
	defer whisper.routingHandler.mutex.RUnlock()

	for peer, status := range whisper.routingHandler.peerStatus {
		if CheckFilterMatch(status.Bloom, envelope.GetBloom()) && envelope.GetPow() >= status.Pow {
			address := whisper.gossiper.GetPeerFromString(peer)
			if address != nil {
				whisper.gossiper.ConnectionHandler.SendPacket(packet, address)
			}
		}
	}
}

// StartRouteRumormongering with the specified timer
func (whisper *Whisper) sendStatusPeriodically() {

	if statusTimer > 0 {

		wPacket := &gossiper.WhisperStatus{Code: statusCode, Pow: whisper.GetMinPow(), Bloom: whisper.GetBloomFilter()}
		whisper.gossiper.SendWhisperStatus(wPacket)

		// start timer
		timer := time.NewTicker(time.Duration(statusTimer) * time.Second)
		for {
			select {
			// rumor monger rumor at each timeout
			case <-timer.C:
				wPacket := &gossiper.WhisperStatus{Code: statusCode, Pow: whisper.GetMinPow(), Bloom: whisper.GetBloomFilter()}
				whisper.gossiper.SendWhisperStatus(wPacket)
			}
		}
	}
}

//// GetOrigins in concurrent environment
//func (gossiper *Gossiper) GetOrigins() []string {
//	gossiper.routingHandler.mutex.RLock()
//	defer gossiper.routingHandler.mutex.RUnlock()
//
//	origins := make([]string, len(gossiper.routingHandler.originLastID.Entries))
//	i := 0
//	for k := range gossiper.routingHandler.originLastID.Entries {
//		origins[i] = k
//		i++
//	}
//	return origins
//}
