package gossiper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// Hash function of BlockPublish
func (b *BlockPublish) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	th := b.Transaction.Hash()
	h.Write(th[:])
	copy(out[:], h.Sum(nil))
	return
}

// Hash function of TXPublish
func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	binary.Write(h, binary.LittleEndian,
		uint32(len(t.Name)))
	h.Write([]byte(t.Name))
	h.Write(t.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}

func (gossiper *Gossiper) qscRound(extPacket *ExtendedGossipPacket) {

	// extPacket.Packet.TLCMessage.TxBlock.PrevHash = gossiper.blHandler.previousBlockHash
	// extPacket.Packet.TLCMessage.Fitness = rand.Float32()
	roundS := gossiper.myTime

	if debug {
		fmt.Println("FITNESS: " + fmt.Sprint(extPacket.Packet.TLCMessage.Fitness))
	}

	if debug {
		fmt.Println("ROUND S")
	}

	// round s
	gossiper.tlcRound(extPacket)

	value, loaded := gossiper.blHandler.confirmations.Load(roundS)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	confirmationsRoundS := value.(map[string]*TLCMessage)

	highestTLCRoundS := gossiper.getTLCWithHighestFitness(confirmationsRoundS)

	if debug {
		fmt.Println("Highest in round s : " + highestTLCRoundS.Origin + " " + fmt.Sprint(highestTLCRoundS.ID) + " " + fmt.Sprint(highestTLCRoundS.Fitness))
	}

	if debug {
		fmt.Println("ROUND S+1")
	}

	// round s + 1
	gossiper.tlcRound(gossiper.createTLCMessage(highestTLCRoundS.TxBlock, -1, highestTLCRoundS.Fitness))

	value, loaded = gossiper.blHandler.confirmations.Load(roundS + 1)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	confirmationsRoundS1 := value.(map[string]*TLCMessage)

	highestTLCRoundS1 := gossiper.getTLCWithHighestFitness(confirmationsRoundS1)

	if debug {
		fmt.Println("Highest in round s : " + highestTLCRoundS1.Origin + " " + fmt.Sprint(highestTLCRoundS1.ID) + " " + fmt.Sprint(highestTLCRoundS1.Fitness))
	}

	if debug {
		fmt.Println("ROUND S+2")
	}

	// round s + 2
	gossiper.tlcRound(gossiper.createTLCMessage(highestTLCRoundS1.TxBlock, -1, highestTLCRoundS1.Fitness))

	value, loaded = gossiper.blHandler.confirmations.Load(roundS + 2)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	if debug {
		fmt.Println("CHECKING IF CONSENSUS REACHED")
	}

	if messageConsensus := checkIfConsensusReached(confirmationsRoundS, confirmationsRoundS1); messageConsensus != nil {

		if debug {
			fmt.Println("CAZZO")
		}

		chosenBlock := messageConsensus.TxBlock
		chosenBlock.PrevHash = gossiper.blHandler.topBlockchainHash
		gossiper.blHandler.committedHistory.Store(chosenBlock.Hash(), chosenBlock)
		gossiper.blHandler.topBlockchainHash = chosenBlock.Hash()
		gossiper.blHandler.previousBlockHash = gossiper.blHandler.topBlockchainHash

		if messageConsensus.Origin == gossiper.name {
			go func(b *BlockPublish) {
				gossiper.uiHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
			}(&chosenBlock)
		}

		gossiper.printConsensusMessage(messageConsensus)

	} else {

		chosenBlock := highestTLCRoundS1.TxBlock
		gossiper.blHandler.previousBlockHash = chosenBlock.Hash()
	}
}

func (gossiper *Gossiper) getTLCWithHighestFitness(confirmations map[string]*TLCMessage) *TLCMessage {

	maxBlock := &TLCMessage{Fitness: 0}
	for _, value := range confirmations {

		if debug {
			fmt.Println(value.Origin + " " + fmt.Sprint(value.ID) + " " + fmt.Sprint(value.Fitness))
		}

		if value.Fitness > maxBlock.Fitness {
			maxBlock = value
		}
	}

	return maxBlock
}

func (gossiper *Gossiper) isTxBlockValid(b BlockPublish) bool {

	isValid := true
	gossiper.blHandler.committedHistory.Range(func(key interface{}, value interface{}) bool {
		block := value.(BlockPublish)

		if block.Transaction.Name == b.Transaction.Name {
			isValid = false
			return false
		}

		return true
	})

	if isValid {
		blockHash := b.PrevHash
		for blockHash != [32]byte{} {
			value, loaded := gossiper.blHandler.committedHistory.Load(blockHash)

			if !loaded {
				isValid = false
				break
			}

			block := value.(BlockPublish)
			blockHash = block.PrevHash
		}
	}

	return isValid
}

func checkIfConsensusReached(confirmationsRoundS, confirmationsRoundS1 map[string]*TLCMessage) *TLCMessage {

	// first condition: m belongs to confirmationsRoundS, i.e. originated from s and was witnessed by round s+1
	for _, message := range confirmationsRoundS {

		condition := false

		// second condition: m witnessed by majority by round s+2
		for _, m := range confirmationsRoundS1 {
			// if m.TxBlock.Transaction.Name == message.TxBlock.Transaction.Name &&
			// 	m.TxBlock.Transaction.Size == message.TxBlock.Transaction.Size &&
			// 	hex.EncodeToString(m.TxBlock.Transaction.MetafileHash) == hex.EncodeToString(message.TxBlock.Transaction.MetafileHash) &&
			// 	m.TxBlock.PrevHash == message.TxBlock.PrevHash &&
			if m.Fitness == message.Fitness {

				condition = true
				break
			}
		}

		if !condition {
			continue
		}

		for _, m0 := range confirmationsRoundS {
			if m0.Fitness > message.Fitness {
				condition = false
				break
			}
		}

		if !condition {
			continue
		}

		for _, m1 := range confirmationsRoundS1 {
			if m1.Fitness > message.Fitness {
				condition = false
				break
			}
		}

		if condition {
			return message
		}
	}

	return nil
}
