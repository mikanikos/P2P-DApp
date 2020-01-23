package whisper

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/mikanikos/Peerster/crypto"
	"golang.org/x/crypto/sha3"
)

// NewKeyPair generates a new cryptographic identity for the client, and injects
// it into the known identities for message decryption. Returns ID of the new key pair.
func (whisper *Whisper) NewKeyPair() (string, error) {
	key, err := crypto.GenerateKey()
	if err != nil || !validatePrivateKey(key){
		return "", err
	}
	if !validatePrivateKey(key) {
		return "", fmt.Errorf("failed to generate valid key")
	}
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

// DeleteKey deletes the specified key if it exists.
func (whisper *Whisper) DeleteKey(key string) {
	whisper.cryptoKeys.Delete(key)
}

// AddKeyPair imports a asymmetric private key and returns it identifier.
func (whisper *Whisper) AddKeyPair(key *ecdsa.PrivateKey) (string, error) {
	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	return id, nil
}

// HasKey checks if the whisper node is configured with the private key
// of the specified public pair.
func (whisper *Whisper) HasKey(id string) bool {
	_, loaded := whisper.cryptoKeys.Load(id)
	return loaded
}

// GetPrivateKey retrieves the private key of the specified identity.
func (whisper *Whisper) GetPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	value, loaded := whisper.cryptoKeys.Load(id)
	if loaded {
		return value.(*ecdsa.PrivateKey), nil
	}
	return nil, fmt.Errorf("no private key for id given")
}

// GenerateSymKey generates a random symmetric key and stores it under id,
// which is then returned. Will be used in the future for session key exchange.
func (whisper *Whisper) GenerateSymKey() (string, error) {
	key, err := generateSecureRandomData(aesKeyLength)
	if err != nil || !validateDataIntegrity(key, aesKeyLength) {
		return "", fmt.Errorf("error in GenerateSymKey: crypto/rand failed to generate random data")
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

// AddSymKeyDirect stores the key, and returns its id.
func (whisper *Whisper) AddSymKeyDirect(key []byte) (string, error) {
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key size: %d", len(key))
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %s", err)
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

//// AddSymKeyFromPassword generates the key from password, stores it, and returns its id.
//func (whisper *Whisper) AddSymKeyFromPassword(password string) (string, error) {
//	id, err := GenerateRandomID()
//	if err != nil {
//		return "", fmt.Errorf("failed to generate ID: %s", err)
//	}
//	if whisper.HasSymKey(id) {
//		return "", fmt.Errorf("failed to generate unique ID")
//	}
//
//	// kdf should run no less than 0.1 seconds on an average computer,
//	// because it's an once in a session experience
//	derived := pbkdf2.Key([]byte(password), nil, 65356, aesKeyLength, sha256.New)
//	if err != nil {
//		return "", err
//	}
//
//	whisper.keyMu.Lock()
//	defer whisper.keyMu.Unlock()
//
//	// double check is necessary, because deriveKeyMaterial() is very slow
//	if whisper.symKeys[id] != nil {
//		return "", fmt.Errorf("critical error: failed to generate unique ID")
//	}
//	whisper.symKeys[id] = derived
//	return id, nil
//}

//// HasSymKey returns true if there is a key associated with the given id.
//// Otherwise returns false.
//func (whisper *Whisper) HasKey(id string) bool {
//	_, loaded := whisper.cryptoKeys.Load(id)
//	return loaded
//}

//// DeleteSymKey deletes the key associated with the name string if it exists.
//func (whisper *Whisper) DeleteSymKey(id string) bool {
//	whisper.keyMu.Lock()
//	defer whisper.keyMu.Unlock()
//	if whisper.symKeys[id] != nil {
//		delete(whisper.symKeys, id)
//		return true
//	}
//	return false
//}

// GetSymKey returns the symmetric key associated with the given id.
func (whisper *Whisper) GetSymKey(id string) ([]byte, error) {
	value, loaded := whisper.cryptoKeys.Load(id)
	if loaded {
		return value.([]byte), nil
	}
	return nil, fmt.Errorf("no sym key for id given")
}

// ValidatePublicKey checks the format of the given public key.
func ValidatePublicKey(k *ecdsa.PublicKey) bool {
	return k != nil && k.X != nil && k.Y != nil && k.X.Sign() != 0 && k.Y.Sign() != 0
}

// validatePrivateKey checks the format of the given private key.
func validatePrivateKey(k *ecdsa.PrivateKey) bool {
	if k == nil || k.D == nil || k.D.Sign() == 0 {
		return false
	}
	return ValidatePublicKey(&k.PublicKey)
}

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	d := sha3.NewLegacyKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}