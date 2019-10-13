package gossiper

import (
	"fmt"
	"net"
)

var modeTypes = []string{"simple", "rumor", "status", "client"}

// SimpleMessage struct
type SimpleMessage struct {
	OriginalName  string
	RelayPeerAddr string
	Contents      string
}

// GossipPacket struct
type GossipPacket struct {
	Simple *SimpleMessage
	Rumor  *RumorMessage
	Status *StatusPacket
}

// NetworkData struct
type NetworkData struct {
	Conn *net.UDPConn
	Addr *net.UDPAddr
}

// ExtendedGossipPacket struct
type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

func initializeChannels(modeTypes []string) (channels map[string]chan *ExtendedGossipPacket) {
	channels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		channels[t] = make(chan *ExtendedGossipPacket)
	}
	return channels
}

func getTypeMode(packet *GossipPacket) string {
	if packet.Simple != nil {
		return "simple"
	}
	if packet.Rumor != nil {
		return "rumor"
	}
	return "status"
}

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.name
}

// IsSimpleMode true or false
func (gossiper *Gossiper) IsSimpleMode() bool {
	return gossiper.simpleMode
}

func (gossiper *Gossiper) printStatusMessage(extPacket *ExtendedGossipPacket) {
	message := "STATUS from " + extPacket.SenderAddr.String() + " "
	for _, value := range extPacket.Packet.Status.Want {
		message = message + "peer " + value.Identifier + " nextID " + fmt.Sprint(value.NextID) + " "
	}
	fmt.Println(message[:len(message)-1])
	gossiper.printPeers()
}

func (gossiper *Gossiper) printPeerMessage(extPacket *ExtendedGossipPacket) {
	if gossiper.simpleMode {
		fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)
	} else {
		fmt.Println("RUMOR origin " + extPacket.Packet.Rumor.Origin + " from " + extPacket.SenderAddr.String() + " ID " + fmt.Sprint(extPacket.Packet.Rumor.ID) + " contents " + extPacket.Packet.Rumor.Text)
	}
	gossiper.printPeers()
}

func (gossiper *Gossiper) printClientMessage(extPacket *ExtendedGossipPacket) {
	if gossiper.simpleMode {
		fmt.Println("CLIENT MESSAGE " + extPacket.Packet.Simple.Contents)
	} else {
		fmt.Println("CLIENT MESSAGE " + extPacket.Packet.Rumor.Text)
	}
	gossiper.printPeers()
}

func (gossiper *Gossiper) modifyPacket(extPacket *ExtendedGossipPacket, isClient bool) *ExtendedGossipPacket {

	newPacket := &ExtendedGossipPacket{SenderAddr: extPacket.SenderAddr, Packet: extPacket.Packet}

	if gossiper.simpleMode {
		simplePacket := &SimpleMessage{OriginalName: extPacket.Packet.Simple.OriginalName, RelayPeerAddr: extPacket.Packet.Simple.RelayPeerAddr, Contents: extPacket.Packet.Simple.Contents}
		if isClient {
			simplePacket.OriginalName = gossiper.GetName()
		}
		simplePacket.RelayPeerAddr = gossiper.gossiperData.Addr.String()
		newPacket.Packet = &GossipPacket{Simple: simplePacket}
	} else {
		rumorPacket := &RumorMessage{ID: extPacket.Packet.Rumor.ID, Origin: extPacket.Packet.Rumor.Origin, Text: extPacket.Packet.Rumor.Text}
		id := gossiper.getIDAtomic()
		gossiper.incerementID()
		rumorPacket.ID = id
		rumorPacket.Origin = gossiper.name
		newPacket.Packet = &GossipPacket{Rumor: rumorPacket}
	}

	return newPacket
}

func (gossiper *Gossiper) notifyStatusChannel(extPacket *ExtendedGossipPacket) {
	_, channelCreated := gossiper.statusChannels[extPacket.SenderAddr.String()]
	if channelCreated {
		go func() {
			gossiper.statusChannels[extPacket.SenderAddr.String()] <- extPacket
		}()
	}
}
