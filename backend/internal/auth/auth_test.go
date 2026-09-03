package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newTokens() *Tokens { return NewTokens("test-secret", time.Hour) }

func TestUserTokenRoundTrip(t *testing.T) {
	tokens := newTokens()

	token, err := tokens.CreateUser("user-123")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	got, err := tokens.ParseUser(token)
	if err != nil {
		t.Fatalf("ParseUser() error = %v", err)
	}
	if got != "user-123" {
		t.Errorf("ParseUser() = %q, want user-123", got)
	}
}

func TestParseUserRejectsBadTokens(t *testing.T) {
	tokens := newTokens()
	other := NewTokens("a-different-secret", time.Hour)

	signedElsewhere, err := other.CreateUser("user-123")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	expired, err := NewTokens("test-secret", -time.Hour).CreateUser("user-123")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	for name, token := range map[string]string{
		"empty":            "",
		"garbage":          "not-a-token",
		"signed elsewhere": signedElsewhere,
		"expired":          expired,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := tokens.ParseUser(token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("ParseUser() error = %v, want ErrInvalidToken", err)
			}
		})
	}
}

// A token that only proves someone knew a room's password must never be
// accepted as proof of who they are.
func TestRoomAccessTokenIsNotAUserToken(t *testing.T) {
	tokens := newTokens()

	access, err := tokens.CreateRoomAccess("ABC123")
	if err != nil {
		t.Fatalf("CreateRoomAccess() error = %v", err)
	}

	if _, err := tokens.ParseUser(access); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ParseUser(room token) error = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyRoomAccess(t *testing.T) {
	tokens := newTokens()

	access, err := tokens.CreateRoomAccess("ABC123")
	if err != nil {
		t.Fatalf("CreateRoomAccess() error = %v", err)
	}

	if !tokens.VerifyRoomAccess(access, "ABC123") {
		t.Error("a room's own access token should be accepted")
	}
	// A token for one room must not open another.
	if tokens.VerifyRoomAccess(access, "XYZ789") {
		t.Error("an access token was accepted for a different room")
	}
	if tokens.VerifyRoomAccess("", "ABC123") {
		t.Error("an empty token was accepted")
	}
	if tokens.VerifyRoomAccess("garbage", "ABC123") {
		t.Error("a malformed token was accepted")
	}
}

// A user token must not double as room access either: the two scopes are
// checked in both directions.
func TestVerifyRoomAccessRejectsAUserToken(t *testing.T) {
	tokens := newTokens()

	userToken, err := tokens.CreateUser("user-123")
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if tokens.VerifyRoomAccess(userToken, "ABC123") {
		t.Error("a user token was accepted as room access")
	}
}

// "alg": "none" is the classic JWT forgery; the parser must insist on HMAC.
func TestParseRejectsUnsignedTokens(t *testing.T) {
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": "user-123",
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("building unsigned token: %v", err)
	}

	if _, err := newTokens().ParseUser(unsigned); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("ParseUser(unsigned) error = %v, want ErrInvalidToken", err)
	}
}

func TestPasswordHashAndVerify(t *testing.T) {
	// The lowest cost bcrypt allows; the tests care about behaviour, not work.
	passwords := NewPasswords(4)

	hash, err := passwords.Hash("hunter2")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if hash == "hunter2" {
		t.Fatal("Hash() returned the password unchanged")
	}

	if !passwords.Verify("hunter2", hash) {
		t.Error("the correct password was rejected")
	}
	if passwords.Verify("wrong", hash) {
		t.Error("an incorrect password was accepted")
	}
}

// Every hash is salted, so the same password never produces the same hash
// twice.
func TestPasswordHashesAreSalted(t *testing.T) {
	passwords := NewPasswords(4)

	first, err := passwords.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	second, err := passwords.Hash("same-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	if first == second {
		t.Error("two hashes of the same password were identical")
	}
}

// An account or room with no password must not be unlockable by supplying an
// empty one.
func TestVerifyAgainstAnEmptyHashAlwaysFails(t *testing.T) {
	passwords := NewPasswords(4)

	if passwords.Verify("", "") {
		t.Error("an empty password matched an empty hash")
	}
	if passwords.Verify("anything", "") {
		t.Error("a password matched an empty hash")
	}
}

// bcrypt cannot hash more than 72 bytes, and silently ignoring the rest would
// make two different long passwords interchangeable.
func TestHashRejectsOverlongPasswords(t *testing.T) {
	if _, err := NewPasswords(4).Hash(strings.Repeat("a", 73)); err == nil {
		t.Error("Hash() accepted a password longer than bcrypt supports")
	}
}

// Hashes written by the previous Python service carry the $2b$ prefix. They
// must keep verifying against this implementation, or the migration would lock
// every existing account out of its own password.
func TestVerifyAcceptsPythonBcryptHashes(t *testing.T) {
	// Produced by Python's bcrypt: bcrypt.hashpw(b"hunter2", bcrypt.gensalt()).
	const pythonHash = "$2b$10$i1m/9fF1tYmq2Xj3Wo4w7OIWqGPne7xrX9SkSRkvf9nvVKpdCngaW"

	passwords := NewPasswords(12)

	if !passwords.Verify("hunter2", pythonHash) {
		t.Error("a hash written by the previous service no longer verifies")
	}
	if passwords.Verify("wrong", pythonHash) {
		t.Error("the wrong password verified against a $2b$ hash")
	}
}
