package utils

import (
	"encoding/hex"

	"golang.org/x/crypto/sha3"
)

// Hash a string and return the first 32 bytes of the hash.
func KeccakHash32(s string) string {
	hash := sha3.NewLegacyKeccak256()

	var buf []byte
	hash.Write([]byte(s))
	buf = hash.Sum(nil)

	encoded := hex.EncodeToString(buf)
	if len(encoded) > 32 {
		encoded = encoded[:32]
	}

	return encoded
}

func GetThreshold(n int) int {
	// t = n * 2/3.
	// Threshold = t - 1
	return (n+1)*2/3 - 1
}
