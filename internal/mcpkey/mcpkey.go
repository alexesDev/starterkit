// Package mcpkey mints the bearer keys an MCP caller presents and derives the
// hash the database keeps instead of them.
package mcpkey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const (
	prefix      = "mcp_"
	randomBytes = 32
)

type Secret string

type Hash []byte

func Generate() (Secret, error) {
	raw := make([]byte, randomBytes)

	_, err := rand.Read(raw)
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	return Secret(prefix + base64.RawURLEncoding.EncodeToString(raw)), nil
}

func (s Secret) Hash() Hash {
	sum := sha256.Sum256([]byte(s))

	return sum[:]
}
