package util

import (
	"bytes"
	"crypto/rand"
	"fmt"
)

// GenerateRandomSHA256 generates a random 64 character SHA 256 hash string
func GenerateRandomSHA256() string {
	return GenerateRandomPrefixedSHA256()[7:]
}

// GenerateRandomPrefixedSHA256 generates a random 64 character SHA 256 hash string, prefixed with `sha256:`
func GenerateRandomPrefixedSHA256() string {
	hash := make([]byte, 32)
	if _, err := rand.Read(hash); err != nil {
		// crypto/rand.Read should never fail under normal circumstances
		// If it does, the system's RNG is broken and we should fail fast
		panic(fmt.Sprintf("failed to generate random data: %v", err))
	}
	sb := bytes.NewBufferString("sha256:")
	sb.Grow(64)
	for _, h := range hash {
		_, _ = fmt.Fprintf(sb, "%02x", h)
	}
	return sb.String()
}
