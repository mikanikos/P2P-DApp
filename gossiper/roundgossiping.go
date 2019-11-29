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

	firstTLCDone := false
	confirmations := make(map[string]uint32)
	witnesses := []string{gossiper.name}

	for block := range gossiper.clientBlockBuffer {

		if !hw3ex3Mode || gossiper.checkAndIncrementRound(confirmations, firstTLCDone) {
			firstTLCDone = false
			confirmations = make(map[string]uint32)
			witnesses = []string{gossiper.name}
		}

		if debug {
			fmt.Println("Processing new file block")
		}

		extPacket := gossiper.createTLCMessage(block, -1)
		id := extPacket.Packet.TLCMessage.ID

		if hw3ex2Mode || hw3ex3Mode {
			printTLCMessage(extPacket.Packet.TLCMessage)
		}

		go gossiper.startRumorMongering(extPacket, gossiper.name, id)

		if stubbornTimeout > 0 {

			if debug {
				fmt.Println("Start waiting for gossiping with confirmation")
			}

			timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
			keepWaiting := true

			for keepWaiting {
				select {
				case tlcAck := <-gossiper.tlcAckChan:
					if tlcAck.ID == id {
						witnesses = append(witnesses, tlcAck.Origin)
						witnesses = helpers.RemoveDuplicatesFromSlice(witnesses)

						if uint64(len(witnesses)) > gossiper.peersNumber/2 && !firstTLCDone {
							timer.Stop()

							gossiper.tlcAckChan = make(chan *TLCAck, maxChannelSize)
							gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(block, int(id)), witnesses)

							keepWaiting = false
							firstTLCDone = true
							confirmations[gossiper.name] = id

							if debug {
								fmt.Println("Sent confirmed, done with this block")
							}

						}
					}

				case tlcConfirm := <-gossiper.tlcConfirmChan:
					confirmations[tlcConfirm.Origin] = tlcConfirm.ID

					if gossiper.checkAndIncrementRound(confirmations, firstTLCDone) {
						timer.Stop()

						fmt.Println("Ok here")

						gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
						gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(block, int(id)), witnesses)

						fmt.Println("Ok here2")

						keepWaiting = false
						firstTLCDone = false
						confirmations = make(map[string]uint32)
						witnesses = []string{gossiper.name}

						if debug {
							fmt.Println("Sent confirmed, done with this block")
						}
					}

				case <-timer.C:
					if hw3ex2Mode || hw3ex3Mode {
						printTLCMessage(extPacket.Packet.TLCMessage)
					}
					go gossiper.startRumorMongering(extPacket, gossiper.name, id)
				}
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

func (gossiper *Gossiper) sendGossipWithConfirmation(extPacket *ExtendedGossipPacket, witnesses []string) {

	if hw3ex2Mode || hw3ex3Mode {
		fmt.Println("RE-BROADCAST ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID) + " WITNESSES " + strings.Join(witnesses, ","))
	}

	go gossiper.startRumorMongering(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

	go func(b *BlockPublish) {
		gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
	}(&extPacket.Packet.TLCMessage.TxBlock)
}

func (gossiper *Gossiper) canAcceptTLCMessage(tlc *TLCMessage) (uint32, bool) {

	gossiper.tlcStatus.Mutex.RLock()
	value, loaded := gossiper.tlcStatus.Status[tlc.Origin]
	gossiper.tlcStatus.Mutex.RUnlock()

	if debug {
		fmt.Println(tlc.VectorClock.Want)
		fmt.Println(gossiper.tlcStatus.Status)
	}

	forMe := isPeerStatusNeeded(tlc.VectorClock.Want, &gossiper.myStatus)
	if !forMe && tlc.Confirmed > -1 {

		if !loaded {
			value = 0
		}

		gossiper.tlcStatus.Mutex.Lock()
		gossiper.tlcStatus.Status[tlc.Origin] = value + 1
		gossiper.tlcStatus.Mutex.Unlock()

		if debug {
			fmt.Println("Vector clock OOOOOOOOOOOOOOOOKKKK!!!")
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

		if hw3ex3Mode {
			printRoundMessage(gossiper.myTime, confirmations)
		}

		return true
	}
	return false
}
