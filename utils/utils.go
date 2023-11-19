package utils

import (
	"io"
	"errors"
	"crypto/rand"
	"encoding/base64"

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
