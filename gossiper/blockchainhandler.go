package gossiper

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// BlockchainHandler struct
type BlockchainHandler struct {
	myTime uint32

	tlcAckChan     chan *TLCAck
	tlcConfirmChan sync.Map

	blockBuffer chan *ExtendedGossipPacket
	tlcStatus   sync.Map

	confirmations sync.Map
	messageSeen   sync.Map

	committedHistory sync.Map

	topBlockchainHash [32]byte
	previousBlockHash [32]byte

	blockchainLogs chan string
}

// NewBlockchainHandler create new blockchain handler
func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{
		myTime:            0,
		tlcAckChan:        make(chan *TLCAck, maxChannelSize),
		tlcConfirmChan:    sync.Map{},
		blockBuffer:       make(chan *ExtendedGossipPacket, maxChannelSize),
		tlcStatus:         sync.Map{},
		confirmations:     sync.Map{},
		messageSeen:       sync.Map{},
		committedHistory:  sync.Map{},
		topBlockchainHash: [32]byte{},
		previousBlockHash: [32]byte{},

		blockchainLogs: make(chan string, 20),
	}
}

// SafeTLCMap struct
type SafeTLCMap struct {
	MessagedIDs map[int]bool
	Mutex       sync.RWMutex
}

// create tx block and start finding an agreement with others
func (gossiper *Gossiper) createAndPublishTxBlock(fileMetadata *FileMetadata) {

	// create tx block
	tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
	block := BlockPublish{Transaction: tx, PrevHash: gossiper.blockchainHandler.previousBlockHash}
	extPacket := gossiper.createTLCMessage(block, -1, rand.Float32())

	// if no simple gossip with confirmation, send it to client block buffer
	if hw3ex2Mode && !hw3ex3Mode && !hw3ex4Mode {
		gossiper.gossipWithConfirmation(extPacket, false)
	} else {
		go func(e *ExtendedGossipPacket) {
			gossiper.blockchainHandler.blockBuffer <- e
		}(extPacket)
	}
}

// process incoming blocks from client (namely new files to be indexed) and choose to use qsc or tlc according to flags
func (gossiper *Gossiper) processClientBlocks() {
	for extPacket := range gossiper.blockchainHandler.blockBuffer {
		if hw3ex4Mode {
			gossiper.qscRound(extPacket)
		} else {
			gossiper.tlcRound(extPacket)
		}
	}
}

// process tlc message
func (gossiper *Gossiper) handleTLCMessage(extPacket *ExtendedGossipPacket) {
	messageRound := gossiper.getMessageOriginalRound(extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID, extPacket.Packet.TLCMessage.Confirmed)
	causalCheck := gossiper.checkForCausalProperty(extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.VectorClock, extPacket.Packet.TLCMessage.Confirmed)

	if debug {
		fmt.Println(extPacket.Packet.TLCMessage.Origin + " is at round " + fmt.Sprint(messageRound))
	}

	// Ack message, depending on flags combination
	if !hw3ex4Mode || gossiper.blockchainHandler.isTxBlockValid(extPacket.Packet.TLCMessage.TxBlock) {
		if !hw3ex3Mode || ackAllMode || uint32(messageRound) >= gossiper.blockchainHandler.myTime {
			if !(extPacket.Packet.TLCMessage.Confirmed > -1) {

				round := atomic.LoadUint32(&gossiper.blockchainHandler.myTime)

				// store message for this round
				value, _ := gossiper.blockchainHandler.messageSeen.LoadOrStore(round, make(map[string]*TLCMessage))
				messages := value.(map[string]*TLCMessage)
				messages[extPacket.Packet.TLCMessage.Origin] = extPacket.Packet.TLCMessage

				// send ack for tlc
				privatePacket := &TLCAck{Origin: gossiper.name, ID: extPacket.Packet.TLCMessage.ID, Destination: extPacket.Packet.TLCMessage.Origin, HopLimit: uint32(hopLimit)}
				if hw3ex2Mode || hw3ex3Mode {
					fmt.Println("SENDING ACK origin " + extPacket.Packet.TLCMessage.Origin + " ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID))
				}
				go gossiper.forwardPrivateMessage(&GossipPacket{Ack: privatePacket}, &privatePacket.HopLimit, privatePacket.Destination)
			}
		}
	}

	if hw3ex3Mode || hw3ex4Mode {
		// check if I can take this confirmation, otherwise process it again later
		if causalCheck && uint32(messageRound) == gossiper.blockchainHandler.myTime {

			if debug {
				fmt.Println("Got confirm for " + fmt.Sprint(extPacket.Packet.TLCMessage.Confirmed) + " from " + extPacket.Packet.TLCMessage.Origin + " " + fmt.Sprint(extPacket.Packet.TLCMessage.Fitness))
			}

			// get channel of the round
			round := atomic.LoadUint32(&gossiper.blockchainHandler.myTime)
			valueChan, _ := gossiper.blockchainHandler.tlcConfirmChan.LoadOrStore(round, make(chan *TLCMessage, maxChannelSize))
			tlcChan := valueChan.(chan *TLCMessage)

			// send message to confirmation channel
			go func(tlc *TLCMessage) {
				tlcChan <- tlc
			}(extPacket.Packet.TLCMessage)
		} else {
			// re-send it to the queue after timeout
			if extPacket.Packet.TLCMessage.Confirmed > -1 {
				go func(ext *ExtendedGossipPacket) {
					time.Sleep(time.Duration(500) * time.Millisecond)
					packetChannels["tlcMes"] <- ext
				}(extPacket)
			}
		}
	}
}

// create tlc message with given paramters
func (gossiper *Gossiper) createTLCMessage(block BlockPublish, confirmedFlag int, fitness float32) *ExtendedGossipPacket {
	id := atomic.LoadUint32(&gossiper.gossipHandler.SeqID)
	atomic.AddUint32(&gossiper.gossipHandler.SeqID, uint32(1))

	tlcPacket := &TLCMessage{Origin: gossiper.name, ID: id, TxBlock: block, VectorClock: gossiper.gossipHandler.MyStatus.createMyStatusPacket(), Confirmed: confirmedFlag, Fitness: fitness}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.connectionHandler.gossiperData.Address}

	// store message
	gossiper.gossipHandler.storeMessage(extPacket.Packet, gossiper.name, id)

	// update vector clock
	tlcPacket.VectorClock = gossiper.gossipHandler.MyStatus.createMyStatusPacket()

	return extPacket
}

// get round origin of the message
func (gossiper *Gossiper) getMessageOriginalRound(origin string, id uint32, confirmedID int) uint32 {
	value, _ := gossiper.blockchainHandler.tlcStatus.LoadOrStore(origin, &SafeTLCMap{MessagedIDs: make(map[int]bool)})
	tlcMap := value.(*SafeTLCMap)

	tlcMap.Mutex.RLock()
	defer tlcMap.Mutex.RUnlock()

	// compute round of the message
	messageRound := uint32(0)
	if confirmedID > -1 {
		for key := range tlcMap.MessagedIDs {
			if confirmedID > key {
				messageRound++
			}
		}
	} else {
		for key := range tlcMap.MessagedIDs {
			if id > uint32(key) {
				messageRound++
			}
		}
	}

	return messageRound
}

// check if message can be accepted for the causal property
func (gossiper *Gossiper) checkForCausalProperty(origin string, receivedVC *StatusPacket, confirmedID int) bool {

	value, _ := gossiper.blockchainHandler.tlcStatus.LoadOrStore(origin, &SafeTLCMap{MessagedIDs: make(map[int]bool)})
	tlcMap := value.(*SafeTLCMap)

	// check vector clock, if v_other[] <= v_mine[] true otherwise false
	vectorClockHigher := gossiper.gossipHandler.MyStatus.checkIfINeedPeerStatus(receivedVC.Want)
	if !vectorClockHigher && confirmedID > -1 {

		if debug {
			fmt.Println("Vector clock OK!")
		}

		tlcMap.Mutex.Lock()
		tlcMap.MessagedIDs[confirmedID] = true
		tlcMap.Mutex.Unlock()

		return true
	}

	return false
}
