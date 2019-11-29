package gossiper

import (
	"encoding/hex"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mikanikos/Peerster/helpers"
)

func (gossiper *Gossiper) processClientBlocks() {

	for block := range gossiper.clientBlockBuffer {

		id := atomic.LoadUint32(&gossiper.seqID)
		atomic.AddUint32(&gossiper.seqID, uint32(1))

		gossiper.checkAndIncrementRound()
		gossiper.updateMyTLCStatusEntry()

		tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block, VectorClock: getStatusToSend(&gossiper.tlcStatus), Confirmed: -1}
		extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.gossiperData.Addr}

		gossiper.storeMessage(extPacket.Packet, gossiper.name, id)

		value, _ := gossiper.tlcChannels.LoadOrStore(id, make(chan *TLCAck))
		tlcChan := value.(chan *TLCAck)

		witnesses := []string{gossiper.name}

		if hw3ex2Mode {
			printTLCMessage(extPacket.Packet.TLCMessage)
		}
		gossiper.startRumorMongering(extPacket, gossiper.name, id)

		if stubbornTimeout > 0 {
			timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
			keepWaiting := true
			for keepWaiting {
				select {
				case tlcAck := <-tlcChan:
					witnesses = append(witnesses, tlcAck.Origin)
					witnesses = helpers.RemoveDuplicatesFromSlice(witnesses)
					if uint64(len(witnesses)) > gossiper.peersNumber/2 && !gossiper.firstTLCDone {
						timer.Stop()
						gossiper.tlcChannels.Delete(id)
						gossiper.sendGossipWithConfirmation(extPacket, id, witnesses)
						keepWaiting = false
						gossiper.firstTLCDone = true
					}

				case <-timer.C:
					if hw3ex2Mode {
						printTLCMessage(extPacket.Packet.TLCMessage)
					}
					if gossiper.checkAndIncrementRound() {
						timer.Stop()
						gossiper.tlcChannels.Delete(id)
						gossiper.sendGossipWithConfirmation(extPacket, id, witnesses)
						keepWaiting = false
					} else {
						gossiper.updateMyTLCStatusEntry()
						go gossiper.startRumorMongering(extPacket, gossiper.name, id)
					}
				}
			}
		}
	}
}

func (gossiper *Gossiper) sendGossipWithConfirmation(extPacket *ExtendedGossipPacket, id uint32, witnesses []string) {
	tlcPacket := extPacket.Packet.TLCMessage
	tlcPacket.Confirmed = int(id)
	newID := atomic.LoadUint32(&gossiper.seqID)
	atomic.AddUint32(&gossiper.seqID, uint32(1))
	tlcPacket.ID = newID

	gossiper.updateMyTLCStatusEntry()

	tlcPacket.VectorClock = getStatusToSend(&gossiper.tlcStatus)

	if hw3ex2Mode {
		fmt.Println("RE-BROADCAST ID " + fmt.Sprint(id) + " WITNESSES " + strings.Join(witnesses, ","))
	}
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)
	go func(b *BlockPublish) {
		gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
	}(&extPacket.Packet.TLCMessage.TxBlock)
}

func (gossiper *Gossiper) canAcceptTLCMessage(tlc *TLCMessage) int {
	gossiper.tlcStatus.Mutex.Lock()
	defer gossiper.tlcStatus.Mutex.Unlock()

	forPeer := getPeerStatusForPeer(tlc.VectorClock.Want, &gossiper.tlcStatus)
	if forPeer == nil {
		forMe := isPeerStatusNeeded(tlc.VectorClock.Want, &gossiper.tlcStatus)
		if !forMe {
			value, loaded := gossiper.tlcStatus.Status[tlc.Origin]
			if !loaded {
				value = 0
			}
			gossiper.tlcStatus.Status[tlc.Origin] = value + 1
			return int(value)
		}
	}

	return -1
}

func (gossiper *Gossiper) updateMyTLCStatusEntry() {
	gossiper.tlcStatus.Mutex.Lock()
	defer gossiper.tlcStatus.Mutex.Unlock()
	gossiper.tlcStatus.Status[gossiper.name] = gossiper.myTime
}

func (gossiper *Gossiper) checkAndIncrementRound() bool {
	if uint64(len(gossiper.confirmations)) > gossiper.peersNumber/2 && gossiper.firstTLCDone {
		atomic.AddUint32(&gossiper.myTime, uint32(1))
		round := atomic.LoadUint32(&gossiper.myTime)
		printRoundMessage(round, gossiper.confirmations)
		gossiper.firstTLCDone = false
		gossiper.confirmations = make(map[string]uint32)
		return true
	}
	return false
}
