package users

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/leenzstra/bebradio/backend/internal/auth"
	"github.com/leenzstra/bebradio/backend/internal/store/memory"
)

func newService() *Service {
	return New(memory.New(), auth.NewTokens("test-secret", time.Hour), auth.NewPasswords(4))
}

func TestRegisterReturnsAUsableToken(t *testing.T) {
	svc := newService()

	token, user, err := svc.Register(t.Context(), "Alice@Example.COM ", " Alice ", "hunter2")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	// The address is stored folded and trimmed, so signing in is not
	// case-sensitive.
	if user.Email != "alice@example.com" {
		t.Errorf("email = %q, want it normalised", user.Email)
	}
	if user.Username != "Alice" {
		t.Errorf("username = %q, want it trimmed", user.Username)
	}
	if len(user.ID) != 8 {
		t.Errorf("id = %q, want 8 characters", user.ID)
	}
	if user.PasswordHash == "hunter2" {
		t.Error("the password was stored unhashed")
	}

	resolved, err := svc.FromToken(t.Context(), token)
	if err != nil {
		t.Fatalf("FromToken() error = %v", err)
	}
	if resolved.ID != user.ID {
		t.Errorf("FromToken() = %q, want %q", resolved.ID, user.ID)
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	svc := newService()
	if _, _, err := svc.Register(t.Context(), "a@b.com", "Alice", "secret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if _, _, err := svc.Register(t.Context(), "A@B.com", "Bob", "secret"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplicate email error = %v, want ErrEmailTaken", err)
	}
	if _, _, err := svc.Register(t.Context(), "c@d.com", "Alice", "secret"); !errors.Is(err, ErrUsernameTaken) {
		t.Errorf("duplicate username error = %v, want ErrUsernameTaken", err)
	}
}

func TestRegisterValidatesInput(t *testing.T) {
	svc := newService()

	cases := map[string][3]string{
		"no at sign":         {"not-an-email", "Alice", "secret"},
		"no dot in domain":   {"alice@localhost", "Alice", "secret"},
		"empty local part":   {"@example.com", "Alice", "secret"},
		"username too short": {"a@b.com", "A", "secret"},
		"username too long":  {"a@b.com", strings.Repeat("x", 31), "secret"},
		"empty password":     {"a@b.com", "Alice", ""},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, err := svc.Register(t.Context(), args[0], args[1], args[2]); !errors.Is(err, ErrInvalidInput) {
				t.Errorf("Register() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	svc := newService()
	if _, _, err := svc.Register(t.Context(), "a@b.com", "Alice", "secret"); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	t.Run("succeeds regardless of address case", func(t *testing.T) {
		_, user, err := svc.Login(t.Context(), " A@B.COM ", "secret")
		if err != nil {
			t.Fatalf("Login() error = %v", err)
		}
		if user.Username != "Alice" {
			t.Errorf("username = %q, want Alice", user.Username)
		}
	})

	// An unknown address and a wrong password must be indistinguishable, or the
	// endpoint would reveal which addresses have accounts.
	t.Run("wrong password", func(t *testing.T) {
		if _, _, err := svc.Login(t.Context(), "a@b.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
		}
	})
	t.Run("unknown address", func(t *testing.T) {
		if _, _, err := svc.Login(t.Context(), "nobody@b.com", "secret"); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("Login() error = %v, want ErrInvalidCredentials", err)
		}
	})
}

func TestFromTokenRejectsRubbish(t *testing.T) {
	svc := newService()

	for _, token := range []string{"", "not-a-token", "a.b.c"} {
		if _, err := svc.FromToken(t.Context(), token); !errors.Is(err, ErrNotFound) {
			t.Errorf("FromToken(%q) error = %v, want ErrNotFound", token, err)
		}
	}
}

func TestUpdateProfileIsPartial(t *testing.T) {
	svc := newService()
	_, user, err := svc.Register(t.Context(), "a@b.com", "Alice", "secret")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	bio := "I like music"
	updated, err := svc.UpdateProfile(t.Context(), user.ID, &bio, nil)
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.Bio != bio {
		t.Errorf("bio = %q, want %q", updated.Bio, bio)
	}

	avatar := "https://img.example/a.png"
	updated, err = svc.UpdateProfile(t.Context(), user.ID, nil, &avatar)
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}
	if updated.Bio != bio {
		t.Errorf("bio = %q, want it left alone by an avatar-only update", updated.Bio)
	}
	if updated.AvatarURL != avatar {
		t.Errorf("avatar_url = %q, want %q", updated.AvatarURL, avatar)
	}
}

func TestUpdateProfileOfAnUnknownAccount(t *testing.T) {
	bio := "hello"
	if _, err := newService().UpdateProfile(t.Context(), "nope", &bio, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateProfile() error = %v, want ErrNotFound", err)
	}
}

// Somebody else's profile must not carry their email address.
func TestUserDTOOmitsEmailForOtherPeople(t *testing.T) {
	svc := newService()
	_, user, err := svc.Register(t.Context(), "a@b.com", "Alice", "secret")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if got := user.DTO(false); got.Email != "" {
		t.Errorf("public profile carried the email address %q", got.Email)
	}
	if got := user.DTO(true); got.Email != "a@b.com" {
		t.Errorf("own profile email = %q, want a@b.com", got.Email)
	}
}
