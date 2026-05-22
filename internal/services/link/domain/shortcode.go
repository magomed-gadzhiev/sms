package domain

import (
	"crypto/rand"
	"math/big"
)

const base62Chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// GenerateCode generates a cryptographically random base62 code of the given length.
func GenerateCode(length int) string {
	b := make([]byte, length)
	max := big.NewInt(int64(len(base62Chars)))
	for i := range b {
		n, _ := rand.Int(rand.Reader, max)
		b[i] = base62Chars[n.Int64()]
	}
	return string(b)
}
