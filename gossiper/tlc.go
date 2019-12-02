package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

func (gossiper *Gossiper) tlcRound(extPacket *ExtendedGossipPacket) {
	round := atomic.LoadUint32(&gossiper.myTime)
	gossiper.blHandler.confirmations.LoadOrStore(round, make(map[string]*TLCMessage))
	gossiper.blHandler.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
	gossiper.gossipWithConfirmation(extPacket, true)
}

func (gossiper *Gossiper) gossipWithConfirmation(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Start gossip with confirmation")
	}

	if hw3ex2Mode {
		gossiper.printTLCMessage(extPacket.Packet.TLCMessage)
	}

	id := extPacket.Packet.TLCMessage.ID
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)

	if stubbornTimeout > 0 {
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
		// used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.name] = 0

		var confirmations map[string]*TLCMessage
		var tlcChan chan *TLCMessage
		if waitConfirmations {

			round := atomic.LoadUint32(&gossiper.myTime)

			value, _ := gossiper.blHandler.confirmations.Load(round)
			confirmations = value.(map[string]*TLCMessage)

			valueChan, _ := gossiper.blHandler.tlcConfirmChan.Load(round)
			tlcChan = valueChan.(chan *TLCMessage)
		}

		delivered := false

		for {
			select {
			case tlcAck := <-gossiper.blHandler.tlcAckChan:

				if tlcAck.ID == id && !delivered {

					witnesses[tlcAck.Origin] = 0
					if uint64(len(witnesses)) > gossiper.peersNumber/2 {
						timer.Stop()

						gossiper.blHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)

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
						gossiper.blHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						if !delivered {
							gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id), extPacket.Packet.TLCMessage.Fitness), getIDForConfirmations(confirmations))
						}
						return
					}
				}

			case <-timer.C:
				if hw3ex2Mode {
					gossiper.printTLCMessage(extPacket.Packet.TLCMessage)
				}
				go gossiper.startRumorMongering(extPacket, gossiper.name, id)
			}
		}
	}
}

func (gossiper *Gossiper) sendGossipWithConfirmation(extPacket *ExtendedGossipPacket, witnesses map[string]uint32) {

	if hw3ex2Mode {
		printConfirmMessage(extPacket.Packet.TLCMessage.ID, witnesses)
	}

	go gossiper.startRumorMongering(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

	if !hw3ex4Mode {
		go func(b *BlockPublish) {
			gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
		}(&extPacket.Packet.TLCMessage.TxBlock)
	}
}

func (gossiper *Gossiper) checkAndIncrementRound(confirmations map[string]*TLCMessage) bool {
	if uint64(len(confirmations)) > gossiper.peersNumber/2 {
		atomic.AddUint32(&gossiper.myTime, uint32(1))
		gossiper.printRoundMessage(gossiper.myTime, confirmations)
		return true
	}
	return false
}
