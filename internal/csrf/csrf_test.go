package csrf

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

func newTokens() *Tokens {
	return New([]byte("test-secret"), time.Hour)
}

func TestTokens_RoundTrip(t *testing.T) {
	tokens := newTokens()

	token := tokens.Generate("session:abc", now)

	require.NoError(t, tokens.Verify(token, "session:abc", now.Add(59*time.Minute)))
}

func TestTokens_Expired(t *testing.T) {
	tokens := newTokens()

	token := tokens.Generate("session:abc", now)

	require.ErrorIs(t, tokens.Verify(token, "session:abc", now.Add(time.Hour+time.Second)), ErrExpired)
}

func TestTokens_WrongBinding(t *testing.T) {
	tokens := newTokens()

	token := tokens.Generate("session:abc", now)

	require.ErrorIs(t, tokens.Verify(token, "session:other", now), ErrMismatch)
}

func TestTokens_TamperedSignature(t *testing.T) {
	tokens := newTokens()
	token := tokens.Generate("session:abc", now)

	last := token[len(token)-1]
	replacement := "0"
	if last == '0' {
		replacement = "1"
	}
	tampered := token[:len(token)-1] + replacement

	require.ErrorIs(t, tokens.Verify(tampered, "session:abc", now), ErrMismatch)
}

func TestTokens_TamperedExpiry(t *testing.T) {
	tokens := newTokens()
	token := tokens.Generate("session:abc", now)

	_, signature, _ := strings.Cut(token, ".")
	extended := "99999999999." + signature

	require.ErrorIs(t, tokens.Verify(extended, "session:abc", now), ErrMismatch)
}

func TestTokens_Malformed(t *testing.T) {
	tokens := newTokens()

	for _, token := range []string{"", "abc", "1.zz", "x.y", ".", "1.", ".abcd"} {
		require.ErrorIs(t, tokens.Verify(token, "session:abc", now), ErrMalformed, token)
	}
}

func TestTokens_OtherSecret(t *testing.T) {
	token := newTokens().Generate("session:abc", now)

	other := New([]byte("another-secret"), time.Hour)

	require.ErrorIs(t, other.Verify(token, "session:abc", now), ErrMismatch)
}
