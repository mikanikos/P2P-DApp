package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

func (gossiper *Gossiper) handleTLCRoundGossiping() {

	confirmations := make(map[string]uint32)
	firstRequestDone := false

	for block := range gossiper.clientBlockBuffer {

		if gossiper.checkAndIncrementRound(confirmations, firstRequestDone) {
			firstRequestDone = false

			for key := range confirmations {
				delete(confirmations, key)
			}

			gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
		}

		gossiper.processClientBlock(block, firstRequestDone, confirmations)

		if !firstRequestDone {
			firstRequestDone = true
		}
	}
}

func (gossiper *Gossiper) processClientBlock(block BlockPublish, firstRequestDone bool, confirmations map[string]uint32) {

	if debug {
		fmt.Println("Processing new block")
	}

	extPacket := gossiper.createTLCMessage(block, -1)
	id := extPacket.Packet.TLCMessage.ID

	if hw3ex2Mode {
		printTLCMessage(extPacket.Packet.TLCMessage)
	}

	go gossiper.startRumorMongering(extPacket, gossiper.name, id)

	if stubbornTimeout > 0 {
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
		// used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.name] = 0

		for {
			select {
			case tlcAck := <-gossiper.tlcAckChan:

				if tlcAck.ID == id {

					witnesses[tlcAck.Origin] = 0
					if uint64(len(witnesses)) > gossiper.peersNumber/2 && !firstRequestDone {
						timer.Stop()

						gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)
						gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(block, int(id)), witnesses)

						confirmations[gossiper.name] = id

						return
					}
				}

			case tlcConfirm := <-gossiper.tlcConfirmChan:

				confirmations[tlcConfirm.Origin] = tlcConfirm.ID
				if gossiper.checkAndIncrementRound(confirmations, firstRequestDone) {
					timer.Stop()

					gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
					gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)

					gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(block, int(id)), confirmations)

					for key := range confirmations {
						delete(confirmations, key)
					}

					return
				}

			case <-timer.C:
				if hw3ex2Mode {
					printTLCMessage(extPacket.Packet.TLCMessage)
				}
				go gossiper.startRumorMongering(extPacket, gossiper.name, id)
			}
		}
	}
}

func (gossiper *Gossiper) createTLCMessage(block BlockPublish, confirmedFlag int) *ExtendedGossipPacket {
	id := atomic.LoadUint32(&gossiper.seqID)
	atomic.AddUint32(&gossiper.seqID, uint32(1))

	tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block, VectorClock: getStatusToSend(&gossiper.myStatus), Confirmed: confirmedFlag}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.gossiperData.Addr}

	gossiper.storeMessage(extPacket.Packet, gossiper.name, id)
	tlcPacket.VectorClock = getStatusToSend(&gossiper.myStatus)

	return extPacket
}

func (gossiper *Gossiper) sendGossipWithConfirmation(extPacket *ExtendedGossipPacket, witnesses map[string]uint32) {

	if hw3ex2Mode {
		printConfirmMessage(extPacket.Packet.TLCMessage.ID, witnesses)
	}

	go gossiper.startRumorMongering(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

	go func(b *BlockPublish) {
		gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
	}(&extPacket.Packet.TLCMessage.TxBlock)
}

func (gossiper *Gossiper) canAcceptTLCMessage(tlc *TLCMessage, isMessageKnown bool) (uint32, bool) {

	gossiper.tlcStatus.Mutex.RLock()
	value, _ := gossiper.tlcStatus.Status[tlc.Origin]
	gossiper.tlcStatus.Mutex.RUnlock()

	// if debug {
	// 	fmt.Println(tlc.VectorClock.Want)
	// 	fmt.Println(gossiper.tlcStatus.Status)
	// }

	vectorClockHigher := isPeerStatusNeeded(tlc.VectorClock.Want, &gossiper.myStatus)
	if !vectorClockHigher && tlc.Confirmed > -1 {

		if debug {
			fmt.Println("Vector clock OOOOOOOOOOOOOOOOKKKK!!!")
		}

		if !isMessageKnown {
			gossiper.tlcStatus.Mutex.Lock()
			gossiper.tlcStatus.Status[tlc.Origin] = value + 1
			gossiper.tlcStatus.Mutex.Unlock()
		}

		return value, true
	}

	if debug {
		fmt.Println("Vector clock NOOO")
	}

	return value, false
}

func (gossiper *Gossiper) updateMyTLCStatusEntry() {
	gossiper.tlcStatus.Mutex.Lock()
	defer gossiper.tlcStatus.Mutex.Unlock()
	gossiper.tlcStatus.Status[gossiper.name] = gossiper.myTime
}

func (gossiper *Gossiper) checkAndIncrementRound(confirmations map[string]uint32, firstTLCDone bool) bool {
	if uint64(len(confirmations)) > gossiper.peersNumber/2 && firstTLCDone {
		atomic.AddUint32(&gossiper.myTime, uint32(1))

		gossiper.updateMyTLCStatusEntry()
		printRoundMessage(gossiper.myTime, confirmations)
		return true
	}
	return false
}
