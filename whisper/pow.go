package whisper

import (
	"encoding/binary"
	gmath "math"
	"math/big"
)

// PoW computes (if necessary) and returns the proof of work target
// of the envelope.
func (e *Envelope) PoW() float64 {
	if e.pow == 0 {
		e.calculatePoW(0)
	}
	return e.pow
}

func (e *Envelope) calculatePoW(diff uint32) {
	rlp := e.rlpWithoutNonce()
	buf := make([]byte, len(rlp)+8)
	copy(buf, rlp)
	binary.BigEndian.PutUint64(buf[len(rlp):], e.Nonce)
	powHash := new(big.Int).SetBytes(Keccak256(buf))
	leadingZeroes := 256 - powHash.BitLen()
	x := gmath.Pow(2, float64(leadingZeroes))
	x /= float64(len(rlp))
	x /= float64(e.TTL + diff)
	e.pow = x
}

func (e *Envelope) powToFirstBit(pow float64) int {
	x := pow
	x *= float64(e.size())
	x *= float64(e.TTL)
	bits := gmath.Log2(x)
	bits = gmath.Ceil(bits)
	res := int(bits)
	if res < 1 {
		res = 1
	}
	return res
}
