package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// start tlc round
func (gossiper *Gossiper) tlcRound(extPacket *ExtendedGossipPacket) {
	// round := atomic.LoadUint32(&gossiper.blHandler.myTime)
	// gossiper.blHandler.confirmations.LoadOrStore(round, make(map[string]*TLCMessage))
	// gossiper.blHandler.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
	gossiper.gossipWithConfirmation(extPacket, true)
}

// gossip with confirmation, wait for confirmation threshold accoridng to flag
func (gossiper *Gossiper) gossipWithConfirmation(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Start gossip with confirmation")
	}

	if hw3ex2Mode {
		gossiper.printPeerMessage(extPacket, gossiper.GetPeersAtomic())
	}

	// start rumor mongering a message
	id := extPacket.Packet.TLCMessage.ID
	go gossiper.startRumorMongering(extPacket, gossiper.name, id)

	if stubbornTimeout > 0 {
		// witnesses (acks), used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.name] = 0

		// confirmations
		var confirmations map[string]*TLCMessage
		var tlcChan chan *TLCMessage
		// get current confirmations and channel
		if waitConfirmations {

			round := atomic.LoadUint32(&gossiper.blockchainHandler.myTime)

			value, _ := gossiper.blockchainHandler.confirmations.LoadOrStore(round, make(map[string]*TLCMessage))
			confirmations = value.(map[string]*TLCMessage)

			valueChan, _ := gossiper.blockchainHandler.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
			tlcChan = valueChan.(chan *TLCMessage)
		}

		delivered := false

		// start timer
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)

		for {
			select {
			// if got ack for this message, record it
			case tlcAck := <-gossiper.blockchainHandler.tlcAckChan:

				if tlcAck.ID == id && !delivered {

					witnesses[tlcAck.Origin] = 0

					// check if received a majority of acks
					if uint64(len(witnesses)) > gossiper.peersData.Size/2 {
						timer.Stop()

						// reset ack channel
						gossiper.blockchainHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						// create tlc confirmed message
						packet := gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id), extPacket.Packet.TLCMessage.Fitness)
						gossiper.sendGossipWithConfirmation(packet, witnesses)
						delivered = true

						// send confirmation
						if waitConfirmations {
							go func(tlc *TLCMessage) {
								tlcChan <- tlc
							}(packet.Packet.TLCMessage)
						} else {
							return
						}
					}
				}

			// if got confirm, record confirmation
			case tlcConfirm := <-tlcChan:

				if waitConfirmations {
					confirmations[tlcConfirm.Origin] = tlcConfirm

					// check if got majority of confirmations and increment round in that case
					if gossiper.checkAndIncrementRound(confirmations) {
						timer.Stop()

						//gossiper.tlcConfirmChan = make(chan *TLCMessage, maxChannelSize)
						// reset confirmation channel
						gossiper.blockchainHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)

						// send gossip with confirmation if not already done
						if !delivered {
							gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(id), extPacket.Packet.TLCMessage.Fitness), getIDForConfirmations(confirmations))
						}
						return
					}
				}

			// periodically resend tlc message
			case <-timer.C:
				if hw3ex2Mode {
					gossiper.printPeerMessage(extPacket, gossiper.GetPeersAtomic())
				}
				go gossiper.startRumorMongering(extPacket, gossiper.name, id)
			}
		}
	}
}

// prepare gossip with confirmation and rumor monger it
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

// check for majority of confirmations and increment round
func (gossiper *Gossiper) checkAndIncrementRound(confirmations map[string]*TLCMessage) bool {
	if uint64(len(confirmations)) > gossiper.peersData.Size/2 {
		atomic.AddUint32(&gossiper.blockchainHandler.myTime, uint32(1))
		gossiper.printRoundMessage(gossiper.blockchainHandler.myTime, confirmations)
		return true
	}
	return false
}
