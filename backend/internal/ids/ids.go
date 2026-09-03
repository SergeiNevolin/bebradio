// Package ids generates the short, URL-friendly identifiers the service hands
// out. The shapes match what the previous service produced (a truncated UUID4),
// so identifiers already stored in the database and shared as room codes stay
// valid.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// New returns n lowercase hexadecimal characters of cryptographically secure
// randomness.
func New(n int) string {
	if n <= 0 {
		return ""
	}
	buf := make([]byte, (n+1)/2)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand never fails on the platforms this service targets; a
		// failure here would mean the OS entropy source is gone, and there is
		// no safe way to continue handing out identifiers.
		panic("ids: reading random bytes: " + err.Error())
	}
	return hex.EncodeToString(buf)[:n]
}

// Room returns a six-character uppercase room code, the format people type in
// to join a room.
func Room() string {
	return strings.ToUpper(New(6))
}

// Short returns the eight-character identifier used for users, tracks and chat
// messages.
func Short() string {
	return New(8)
}
