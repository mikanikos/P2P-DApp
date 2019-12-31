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

	if debug {
		fmt.Println("Done tlc round")
	}

}

// gossip with confirmation, wait for confirmation threshold accoridng to flag
func (gossiper *Gossiper) gossipWithConfirmation(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Start gossip with confirmation")
	}

	if hw3ex2Mode {
		gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
	}

	// start rumor mongering a message
	tlc := extPacket.Packet.TLCMessage

	gossiper.blockchainHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)
	go gossiper.startRumorMongering(extPacket, gossiper.name, tlc.ID)


	if waitConfirmations && gossiper.checkForConfirmationsMajority(tlc, true) {	
		return
	}

	if stubbornTimeout > 0 {
		// witnesses (acks), used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.name] = 0
		delivered := false

		// start timer
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
		defer timer.Stop()
		for {
			select {
			// if got ack for this message, record it
			case tlcAck := <-gossiper.blockchainHandler.tlcAckChan:

				if tlcAck.ID == tlc.ID && !delivered {
					witnesses[tlcAck.Origin] = 0

					// check if received a majority of acks
					if len(witnesses) > int(gossiper.peersData.Size/2) {
		
						// create tlc confirmed message
						gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(extPacket.Packet.TLCMessage.TxBlock, int(tlc.ID), extPacket.Packet.TLCMessage.Fitness), witnesses)
						delivered = true

						// send confirmation
						if waitConfirmations {
							gossiper.blockchainHandler.saveConfirmation(atomic.LoadUint32(&gossiper.blockchainHandler.myTime), extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage)

							go func() {
								gossiper.blockchainHandler.tlcConfirmChan <- true
							}()

						} else {
							return
						}
					}
				}

			case <-gossiper.blockchainHandler.tlcConfirmChan:
				if waitConfirmations && gossiper.checkForConfirmationsMajority(tlc, delivered) {
					return
				}

			// periodically resend tlc message
			case <-timer.C:

				// if waitConfirmations && gossiper.checkForConfirmationsMajority(tlc, true) {	
				// 	return
				// }

				if hw3ex2Mode {
					gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
				}
				go gossiper.startRumorMongering(extPacket, gossiper.name, tlc.ID)
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
			gossiper.fileHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
		}(&extPacket.Packet.TLCMessage.TxBlock)
	}
}

// check for majority of confirmations and increment round
func (gossiper *Gossiper) checkForConfirmationsMajority(tlc *TLCMessage, delivered bool) bool {
	round := atomic.LoadUint32(&gossiper.blockchainHandler.myTime)
	value, loaded := gossiper.blockchainHandler.confirmations.Load(round)

	if loaded {

		tlcMap := value.(*SafeTLCMessagesMap)

		tlcMap.Mutex.RLock()
		confirmations := tlcMap.OriginMessage
		tlcMap.Mutex.RUnlock()

		if debug {
			fmt.Println("Got: " + fmt.Sprint(len(confirmations)))
			fmt.Println(confirmations)
		}

		// check if got majority of confirmations and increment round in that case
		if len(confirmations) > int(gossiper.peersData.Size/2) {

			atomic.AddUint32(&gossiper.blockchainHandler.myTime, uint32(1))
			gossiper.printRoundMessage(gossiper.blockchainHandler.myTime, confirmations)

			if !delivered {
				gossiper.sendGossipWithConfirmation(gossiper.createTLCMessage(tlc.TxBlock, int(tlc.ID), tlc.Fitness), getIDForConfirmations(confirmations))
			}

			return true
		}
	}

	return false
}
