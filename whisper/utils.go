package whisper

import (
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateRandomID generates a random string
func GenerateRandomID() (id string, err error) {
	buf, err := generateRandomBytes(keyIDSize)
	if err != nil {
		return "", err
	}
	if len(buf) != keyIDSize {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = hex.EncodeToString(buf)
	return id, err
}

func generateRandomBytes(length int) ([]byte, error) {
	array := make([]byte, length)
	_, err := crand.Read(array)
	if err != nil {
		return nil, err
	}
	return array, nil
}
