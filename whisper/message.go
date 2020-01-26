package whisper

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/dedis/protobuf"
	ecies "github.com/ecies/go"
	"golang.org/x/crypto/sha3"
	"io"
	"math/big"
	"time"
)

// MessageParams specifies all the parameters needed to prepare an envelope
type MessageParams struct {
	Src     *ecies.PrivateKey
	Dst     *ecies.PublicKey
	KeySym  []byte
	Topic   Topic
	PowTime uint32
	TTL     uint32
	Payload []byte
	Padding []byte
}

// SentMessage represents an end-user data packet to transmit through the
// Whisper protocol. These are wrapped into Envelopes that need not be
// understood by intermediate nodes, just forwarded.
//type sentMessage struct {
//	Raw []byte
//}

//ReceivedMessage represents a message received and decrypted
type ReceivedMessage struct {
	// Raw []byte

	//Payload   []byte
	//Padding   []byte
	//Signature []byte
	//Salt      []byte

	Sent    uint32
	TTL     uint32
	Src     *ecies.PublicKey
	Dst     *ecies.PublicKey
	Payload []byte
	Topic   Topic

	SymKeyHash   [32]byte
	EnvelopeHash [32]byte
}

//func (msg *ReceivedMessage) isSymmetricEncryption() bool {
//	return msg.SymKeyHash != [32]byte{}
//}
//
//func (msg *ReceivedMessage) isAsymmetricEncryption() bool {
//	return msg.Dst != nil
//}

// NewSentMessage creates and initializes a non-signed, non-encrypted Whisper message.
//func NewSentMessage(params *MessageParams) (*sentMessage, error) {
//	const payloadSizeFieldMaxSize = 4
//	msg := sentMessage{}
//	msg.Raw = make([]byte, 1, flagsLength+payloadSizeFieldMaxSize+len(params.Payload)+len(params.Padding)+padSizeLimit)
//	msg.Raw[0] = 0 // set all the flags to zero
//	msg.addPayloadSizeField(params.Payload)
//	msg.Raw = append(msg.Raw, params.Payload...)
//	err := msg.appendPadding(params)
//	return &msg, err
//}
//
//// addPayloadSizeField appends the auxiliary field containing the size of payload
//func (msg *sentMessage) addPayloadSizeField(payload []byte) {
//	fieldSize := len(payload)
//	field := make([]byte, 4)
//	binary.LittleEndian.PutUint32(field, uint32(len(payload)))
//	field = field[:fieldSize]
//	msg.Raw = append(msg.Raw, field...)
//	msg.Raw[0] |= byte(fieldSize)
//}

// getSizeOfPayloadSizeField returns the number of bytes necessary to encode the size of payload
//func getSizeOfPayloadSizeField(payload []byte) int {
//	s := 1
//	for i := len(payload); i >= 256; i /= 256 {
//		s++
//	}
//	return s
//}

// appendPadding appends random padding to the message
func (params *MessageParams) appendPadding() error {
	bytes, err := protobuf.Encode(params)
	if err != nil {
		return err
	}
	rawSize := len(bytes)
	odd := rawSize % padSizeLimit
	paddingSize := padSizeLimit - odd

	pad := make([]byte, paddingSize)
	_, err = crand.Read(pad)
	if err != nil {
		return err
	}
	params.Payload = pad
	//msg.Raw = append(msg.Raw, pad...)
	return nil
}

// sign calculates and sets the cryptographic signature for the message,
// also setting the sign flag.
//func (msg *sentMessage) sign(key *ecies.PrivateKey) error {
//	if isMessageSigned(msg.Raw[0]) {
//		// this should not happen, but no reason to panic
//		fmt.Println("failed to sign the message: already signed")
//		return nil
//	}
//
//	msg.Raw[0] |= signatureFlag // it is important to set this flag before signing
//	hash := crypto.Keccak256(msg.Raw)
//	signature, err := crypto.Sign(hash, key)
//	if err != nil {
//		msg.Raw[0] &= (0xFF ^ signatureFlag) // clear the flag
//		return err
//	}
//	msg.Raw = append(msg.Raw, signature...)
//	return nil
//}

// encryptWithPublicKey encrypts a message with a public key.
func encryptWithPublicKey(data []byte, key *ecies.PublicKey) ([]byte, error) {
	return ecies.Encrypt(key, data)
}

// use aes-gcm
func encryptWithSymmetricKey(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(crand.Reader, nonce)
	if err != nil {
		return nil, err
	}
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

// GetEnvelopeFromMessage encrypts the message and prepare the Envelope
func (params *MessageParams) GetEnvelopeFromMessage() (envelope *Envelope, err error) {
	if params.TTL == 0 {
		params.TTL = DefaultTTL
	}

	var encrypted []byte

	if params.Dst != nil {
		encrypted, err = encryptWithPublicKey(params.Payload, params.Dst)
		if err != nil {
			return nil, err
		}
	} else if params.KeySym != nil {
		encrypted, err = encryptWithSymmetricKey(params.Payload, params.KeySym)
		if err != nil {
			return nil, err
		}
	} else {
		err = fmt.Errorf("no symmetric or asymmetric key given")
	}
	if err != nil {
		return nil, err
	}

	envelope = NewEnvelope(params.TTL, params.Topic, encrypted)
	//
	//fmt.Println("CAZZOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOOO")
	//fmt.Println(string(params.Payload))
	//fmt.Println("MAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")

	// add nonce for requiring enough pow
	envelope.addNonceForPow(params)

	return envelope, nil
}

func (e *Envelope) addNonceForPow(options *MessageParams) {

	e.Expiry += options.PowTime

	bestLeadingZeros := 0

	encodedEnvWithoutNonce, _ := protobuf.Encode([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	buf := make([]byte, len(encodedEnvWithoutNonce)+8)
	copy(buf, encodedEnvWithoutNonce)

	finish := time.Now().Add(time.Duration(options.PowTime) * time.Second).UnixNano()
	for nonce := uint64(0); time.Now().UnixNano() < finish; {
		for i := 0; i < 1024; i++ {
			binary.BigEndian.PutUint64(buf[len(encodedEnvWithoutNonce):], nonce)
			d := sha3.New256()
			d.Write(buf)
			powHash := new(big.Int).SetBytes(d.Sum(nil))
			leadingZeros := 256 - powHash.BitLen()
			if leadingZeros > bestLeadingZeros {
				e.Nonce, bestLeadingZeros = nonce, leadingZeros
			}
			nonce++
		}
	}

	e.computePow(0)
}

// decryptWithSymmetricKey decrypts a message with symmetric key
func decryptWithSymmetricKey(encrypted []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

// decryptWithPrivateKey decrypts an encrypted payload with private key
func decryptWithPrivateKey(encrypted []byte, key *ecies.PrivateKey) ([]byte, error) {
	return ecies.Decrypt(key, encrypted)
}

// ValidateAndParse checks the message validity and extracts the fields in case of success.
//func (msg *ReceivedMessage) ValidateAndParse() bool {
//	end := len(msg.Raw)
//	if end < 1 {
//		return false
//	}
//
//	if isMessageSigned(msg.Raw[0]) {
//		end -= signatureLength
//		if end <= 1 {
//			return false
//		}
//		msg.Signature = msg.Raw[end : end+signatureLength]
//		msg.Src = msg.SigToPubKey()
//		if msg.Src == nil {
//			return false
//		}
//	}
//
//	beg := 1
//	payloadSize := 0
//	sizeOfPayloadSizeField := int(msg.Raw[0] & SizeMask) // number of bytes indicating the size of payload
//	if sizeOfPayloadSizeField != 0 {
//		payloadSize = int(bytesToUintLittleEndian(msg.Raw[beg : beg+sizeOfPayloadSizeField]))
//		if payloadSize+1 > end {
//			return false
//		}
//		beg += sizeOfPayloadSizeField
//		msg.Payload = msg.Raw[beg : beg+payloadSize]
//	}
//
//	beg += payloadSize
//	msg.Padding = msg.Raw[beg:end]
//	return true
//}

// SigToPubKey returns the public key associated to the message's
// signature.
//func (msg *ReceivedMessage) SigToPubKey() *ecies.PublicKey {
//	defer func() { recover() }() // in case of invalid signature
//
//	pub, err := crypto.SigToPub(msg.hash(), msg.Signature)
//	if err != nil {
//		fmt.Println("failed to recover public key from signature", "err", err)
//		return nil
//	}
//	return pub
//}
