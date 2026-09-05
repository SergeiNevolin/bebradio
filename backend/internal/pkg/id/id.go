package id

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func New(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	var sb strings.Builder
	for _, v := range b {
		sb.WriteByte(charset[v%byte(len(charset))])
	}
	return sb.String()
}

func NewHex(length int) string {
	b := make([]byte, (length+1)/2)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func NewUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}
