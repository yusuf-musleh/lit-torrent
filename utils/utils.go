package utils

import (
	"io"
	"errors"
	"crypto/rand"
	"encoding/base64"
	"sync"

	bencode "github.com/jackpal/bencode-go"
)

// Generate random string with provided length
func GenerateRandomString(length int) (string, error) {
	buffer := make([]byte, length)
	_, err := rand.Read(buffer)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(buffer)[:length], nil
}

// Parse and decode response body from Tracker
func ParseBencodeResponse(body io.ReadCloser) (map[string]interface{}, error) {
	decoded, decodeErr := bencode.Decode(body)
	if decodeErr != nil {
		return nil, decodeErr
	}

	data, ok := decoded.(map[string]interface{})
	if ok {
		return data, nil
	}

	return nil, errors.New("Bencode type mismatch")
}

type PeerCount struct {
	Mu		*sync.Mutex
	Count	int
}

// Safely increment peers count
func (pc *PeerCount) Increment() {
	pc.Mu.Lock()
	pc.Count++
	pc.Mu.Unlock()
}

// Safely decrement peers count
func (pc *PeerCount) Decrement() {
	pc.Mu.Lock()
	pc.Count--
	pc.Mu.Unlock()
}

// Safely get peers count
func (pc *PeerCount) GetCount() int {
	pc.Mu.Lock()
	currentCount := pc.Count
	pc.Mu.Unlock()
	return currentCount
}
