package gossiper

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
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

	extPacket.Packet.TLCMessage.Fitness = rand.Float32()
	roundS := gossiper.myTime

	// round s
	gossiper.qscRound(extPacket)

	value, loaded := gossiper.confirmations.Load(roundS)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	confirmationsRoundS := value.(map[string]*TLCMessage)

	highestTLCRoundS := gossiper.getTLCWithHighestFitness(confirmationsRoundS)

	// round s + 1
	gossiper.qscRound(gossiper.createTLCMessage(highestTLCRoundS.TxBlock, -1))

	value, loaded = gossiper.confirmations.Load(roundS + 1)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	confirmationsRoundS1 := value.(map[string]*TLCMessage)

	highestTLCRoundS1 := gossiper.getTLCWithHighestFitness(confirmationsRoundS1)

	// round s + 2
	gossiper.qscRound(gossiper.createTLCMessage(highestTLCRoundS1.TxBlock, -1))

	value, loaded = gossiper.confirmations.Load(roundS + 2)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	if messageConsensus := checkIfConsensusReached(confirmationsRoundS, confirmationsRoundS1); messageConsensus != nil {

		chosenBlock := messageConsensus.TxBlock
		chosenBlock.PrevHash = gossiper.topBlockchainHash
		gossiper.topBlockchainHash = chosenBlock.Hash()

		gossiper.printConsensusMessage(messageConsensus)

	} else {
		chosenBlock := highestTLCRoundS1.TxBlock
		chosenBlock.PrevHash = gossiper.topBlockchainHash
		gossiper.topBlockchainHash = chosenBlock.Hash()
	}
}

func (gossiper *Gossiper) getTLCWithHighestFitness(confirmations map[string]*TLCMessage) *TLCMessage {

	maxBlock := confirmations[gossiper.name]
	for _, value := range confirmations {
		if value.Fitness > maxBlock.Fitness {
			maxBlock = value
		}
	}
	return maxBlock
}

func (gossiper *Gossiper) isTxBlockValid(b BlockPublish) bool {

	isValid := true
	gossiper.committedHistory.Range(func(key interface{}, value interface{}) bool {
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
			value, loaded := gossiper.committedHistory.Load(blockHash)

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
			if m.Origin == message.Origin && m.Confirmed == message.Confirmed {
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
