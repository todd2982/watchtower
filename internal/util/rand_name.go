package util

import (
	"crypto/rand"
	"fmt"
)

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandName generates a random, 32-character, Docker-compatible container name using cryptographic randomness.
// Returns an error if random bytes cannot be generated.
func RandName() (string, error) {
	b := make([]rune, 32)
	randomBytes := make([]byte, 32)

	// Use crypto/rand for secure randomness
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Map random bytes to allowed letters
	for i := range b {
		b[i] = letters[int(randomBytes[i])%len(letters)]
	}

	return string(b), nil
}
