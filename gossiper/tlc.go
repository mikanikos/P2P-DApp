package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

func (gossiper *Gossiper) handleTLCMessage(extPacket *ExtendedGossipPacket) {
	messageRound, canTake := gossiper.canAcceptTLCMessage(extPacket.Packet.TLCMessage)

	if debug {
		fmt.Println(extPacket.Packet.TLCMessage.Origin + " is at round " + fmt.Sprint(messageRound))
	}

	// SHOULD I FILTER MESSAGES BEFORE, ACCORDING TO THE PEER ROUND / VECTOR CLOCK?

	// Ack message
	if !hw3ex4Mode || gossiper.isTxBlockValid(extPacket.Packet.TLCMessage.TxBlock) {
		if !hw3ex3Mode || ackAllMode || uint32(messageRound) >= gossiper.myTime {
			if !(extPacket.Packet.TLCMessage.Confirmed > -1) {
				privatePacket := &TLCAck{Origin: gossiper.name, ID: extPacket.Packet.TLCMessage.ID, Destination: extPacket.Packet.TLCMessage.Origin, HopLimit: uint32(hopLimit)}
				if hw3ex2Mode || hw3ex3Mode {
					fmt.Println("SENDING ACK origin " + extPacket.Packet.TLCMessage.Origin + " ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID))
				}
				go gossiper.forwardPrivateMessage(&GossipPacket{Ack: privatePacket}, &privatePacket.HopLimit, privatePacket.Destination)
			}
		}
	} else {
		if debug {
			fmt.Println("ERROR IN TX VALIDATION ")
		}
	}

	if hw3ex3Mode {
		if canTake && uint32(messageRound) == gossiper.myTime {

			if debug {
				fmt.Println("Got confirm for " + fmt.Sprint(extPacket.Packet.TLCMessage.Confirmed) + " from " + extPacket.Packet.TLCMessage.Origin + " " + fmt.Sprint(extPacket.Packet.TLCMessage.Fitness))
			}

			round := atomic.LoadUint32(&gossiper.myTime)
			valueChan, _ := gossiper.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
			tlcChan := valueChan.(chan *TLCMessage)

			go func(tlc *TLCMessage) {
				tlcChan <- tlc
			}(extPacket.Packet.TLCMessage)
		} else {
			if extPacket.Packet.TLCMessage.Confirmed > -1 {
				go func(ext *ExtendedGossipPacket) {
					time.Sleep(time.Duration(500) * time.Millisecond)
					gossiper.channels["tlcMes"] <- ext
				}(extPacket)
			}
		}
	}
}

func (gossiper *Gossiper) tlcRound(extPacket *ExtendedGossipPacket) {
	round := atomic.LoadUint32(&gossiper.myTime)
	gossiper.confirmations.LoadOrStore(round, make(map[string]*TLCMessage))
	gossiper.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
	gossiper.gossipWithConfirmation(extPacket, true)
}

func (gossiper *Gossiper) gossipWithConfirmation(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Start gossip with confirmation")
	}

	if hw3ex2Mode {
		printTLCMessage(extPacket.Packet.TLCMessage)
	}

	id := extPacket.Packet.TLCMessage.ID
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)

	if stubbornTimeout > 0 {
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
		// used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.name] = 0

		round := atomic.LoadUint32(&gossiper.myTime)

		value, _ := gossiper.confirmations.Load(round)
		confirmations := value.(map[string]*TLCMessage)

		valueChan, _ := gossiper.tlcConfirmChan.Load(round)
		tlcChan := valueChan.(chan *TLCMessage)

		delivered := false

		for {
			select {
			case tlcAck := <-gossiper.tlcAckChan:

				if tlcAck.ID == id && !delivered {

					witnesses[tlcAck.Origin] = 0
					if uint64(len(witnesses)) > gossiper.peersNumber/2 {
						timer.Stop()

						gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						packet := gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id), extPacket.Packet.TLCMessage.Fitness)
						gossiper.sendGossipWithConfirmation(packet, witnesses)
						delivered = true

						if waitConfirmations {
							go func(tlc *TLCMessage) {
								tlcChan <- tlc
							}(packet.Packet.TLCMessage)
						} else {
							return
						}
					}
				}

			case tlcConfirm := <-tlcChan:

				if waitConfirmations {
					confirmations[tlcConfirm.Origin] = tlcConfirm
					if gossiper.checkAndIncrementRound(confirmations) {
						timer.Stop()

						//gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
						gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						if !delivered {
							gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id), extPacket.Packet.TLCMessage.Fitness), getIDForConfirmations(confirmations))
						}
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

func getIDForConfirmations(confirmations map[string]*TLCMessage) map[string]uint32 {
	originIDMap := make(map[string]uint32)
	for key, value := range confirmations {
		originIDMap[key] = value.ID
	}
	return originIDMap
}

func (gossiper *Gossiper) createTLCMessage(block BlockPublish, confirmedFlag int, fitness float32) *ExtendedGossipPacket {
	id := atomic.LoadUint32(&gossiper.seqID)
	atomic.AddUint32(&gossiper.seqID, uint32(1))

	tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block, VectorClock: getStatusToSend(&gossiper.myStatus), Confirmed: confirmedFlag, Fitness: fitness}
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

func (gossiper *Gossiper) checkAndIncrementRound(confirmations map[string]*TLCMessage) bool {
	if uint64(len(confirmations)) > gossiper.peersNumber/2 {
		atomic.AddUint32(&gossiper.myTime, uint32(1))
		printRoundMessage(gossiper.myTime, confirmations)
		return true
	}
	return false
}
