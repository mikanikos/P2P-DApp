package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func (gossiper *Gossiper) handleTLCMessage(extPacket *ExtendedGossipPacket, isMessageKnown bool) {
	messageRound, canTake := gossiper.canAcceptTLCMessage(extPacket.Packet.TLCMessage)

	if debug {
		fmt.Println(extPacket.Packet.TLCMessage.Origin + " is at round " + fmt.Sprint(messageRound))
	}

	// SHOULD I FILTER MESSAGES BEFORE, ACCORDING TO THE PEER ROUND / VECTOR CLOCK?

	// Ack message
	if !(extPacket.Packet.TLCMessage.Confirmed > -1) && (!hw3ex3Mode || ackAllMode || uint32(messageRound) >= gossiper.myTime) {
		privatePacket := &TLCAck{Origin: gossiper.name, ID: extPacket.Packet.TLCMessage.ID, Destination: extPacket.Packet.TLCMessage.Origin, HopLimit: uint32(hopLimit)}
		if hw3ex2Mode || hw3ex3Mode {
			fmt.Println("SENDING ACK origin " + extPacket.Packet.TLCMessage.Origin + " ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID))
		}
		go gossiper.forwardPrivateMessage(&GossipPacket{Ack: privatePacket}, &privatePacket.HopLimit, privatePacket.Destination)
	}

	if hw3ex3Mode {
		if canTake && uint32(messageRound) == gossiper.myTime {

			if debug {
				fmt.Println("Got confirm for " + fmt.Sprint(extPacket.Packet.TLCMessage.Confirmed) + " from " + extPacket.Packet.TLCMessage.Origin)
			}

			go func(tlc *TLCMessage) {
				gossiper.tlcConfirmChan <- tlc
			}(extPacket.Packet.TLCMessage)
		} else {
			if !isMessageKnown && extPacket.Packet.TLCMessage.Confirmed > -1 {
				go func(ext *ExtendedGossipPacket) {
					time.Sleep(time.Duration(500) * time.Millisecond)
					gossiper.channels["tlcMes"] <- ext
				}(extPacket)
			}
		}
	}
}

func (gossiper *Gossiper) handleTLCRoundGossiping() {

	// confirmations := make(map[string]uint32)
	// firstRequestDone := false

	for extPacket := range gossiper.tlcBuffer {

		// if gossiper.checkAndIncrementRound(confirmations, firstRequestDone) {
		// 	firstRequestDone = false

		// 	for key := range confirmations {
		// 		delete(confirmations, key)
		// 	}

		// 	gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
		// }

		fmt.Println(gossiper.myTime)
		gossiper.confirmations.Store(gossiper.myTime, make(map[string]uint32))
		gossiper.processClientBlock(extPacket, true)

		// if !firstRequestDone {
		// 	firstRequestDone = true
		// }
	}
}

func (gossiper *Gossiper) processClientBlock(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Processing new block")
	}

	//extPacket := gossiper.createTLCMessage(block, -1)
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

		value, _ := gossiper.confirmations.Load(gossiper.myTime)
		confirmations := value.(map[string]uint32)
		delivered := false

		for {
			select {
			case tlcAck := <-gossiper.tlcAckChan:

				if tlcAck.ID == id && !delivered {

					witnesses[tlcAck.Origin] = 0
					if uint64(len(witnesses)) > gossiper.peersNumber/2 {
						timer.Stop()

						gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						packet := gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id))
						gossiper.sendGossipWithConfirmation(packet, witnesses)
						delivered = true

						if waitConfirmations {
							go func(tlc *TLCMessage) {
								gossiper.tlcConfirmChan <- tlc
							}(packet.Packet.TLCMessage)
						} else {
							return
						}
					}
				}

			case tlcConfirm := <-gossiper.tlcConfirmChan:

				if waitConfirmations {
					fmt.Println(confirmations)
					confirmations[tlcConfirm.Origin] = tlcConfirm.ID
					if gossiper.checkAndIncrementRound(confirmations) {
						timer.Stop()

						gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
						gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						if !delivered {
							gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id)), confirmations)
						}
						// for key := range confirmations {
						// 	delete(confirmations, key)
						// }

						return
					}
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

// SafeTLCMap struct
type SafeTLCMap struct {
	MessagedIDs map[int]bool
	Mutex       sync.RWMutex
}

func (gossiper *Gossiper) canAcceptTLCMessage(tlc *TLCMessage) (uint32, bool) {

	value, _ := gossiper.tlcStatus.LoadOrStore(tlc.Origin, &SafeTLCMap{MessagedIDs: make(map[int]bool)})
	tlcMap := value.(*SafeTLCMap)

	tlcMap.Mutex.RLock()

	messageRound := uint32(0)
	if tlc.Confirmed > -1 {
		for key := range tlcMap.MessagedIDs {
			if tlc.Confirmed > key {
				messageRound++
			}
		}
	} else {
		for key := range tlcMap.MessagedIDs {
			if tlc.ID > uint32(key) {
				messageRound++
			}
		}
	}

	tlcMap.Mutex.RUnlock()

	vectorClockHigher := isPeerStatusNeeded(tlc.VectorClock.Want, &gossiper.myStatus)
	if !vectorClockHigher && tlc.Confirmed > -1 {

		if debug {
			fmt.Println("Vector clock OOOOOOOOOOOOOOOOKKKK!!!")
		}

		tlcMap.Mutex.Lock()
		tlcMap.MessagedIDs[tlc.Confirmed] = true
		tlcMap.Mutex.Unlock()

		return messageRound, true
	}

	return messageRound, false
}

func (gossiper *Gossiper) checkAndIncrementRound(confirmations map[string]uint32) bool {
	if uint64(len(confirmations)) > gossiper.peersNumber/2 {
		atomic.AddUint32(&gossiper.myTime, uint32(1))
		printRoundMessage(gossiper.myTime, confirmations)
		return true
	}
	return false
}
