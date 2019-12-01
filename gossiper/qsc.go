package gossiper

import (
	"crypto/sha256"
	"encoding/binary"
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

// func (gossiper *Gossiper) handleQSCRound() {

// 	for extPacket := range gossiper.qscBuffer {

// 		extPacket.Packet.TLCMessage.Fitness = rand.Float32()

// 		round_s := gossiper.myTime

// 		go func(e *ExtendedGossipPacket) {
// 			gossiper.tlcBuffer <- e
// 		}(extPacket)

// 		for round_s + 1 != gossiper.myTime
// 	}
// }
