package csrf

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMalformed = errors.New("malformed csrf token")
	ErrExpired   = errors.New("csrf token expired")
	ErrMismatch  = errors.New("csrf token mismatch")
)

// Tokens выпускает и проверяет stateless CSRF-токены вида
// "<exp_unix>.<hex(HMAC-SHA256(secret, binding|exp_unix))>".
// binding — значение, к которому привязан токен (например, id сессии).
type Tokens struct {
	secret []byte
	ttl    time.Duration
}

func New(secret []byte, ttl time.Duration) *Tokens {
	return &Tokens{secret: secret, ttl: ttl}
}

func (t *Tokens) Generate(binding string, now time.Time) string {
	exp := strconv.FormatInt(now.Add(t.ttl).Unix(), 10)
	return exp + "." + hex.EncodeToString(t.sign(binding, exp))
}

func (t *Tokens) Verify(token, binding string, now time.Time) error {
	exp, signatureHex, ok := strings.Cut(token, ".")
	if !ok {
		return ErrMalformed
	}

	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return ErrMalformed
	}

	signature, err := hex.DecodeString(signatureHex)
	if err != nil || len(signature) != sha256.Size {
		return ErrMalformed
	}

	if !hmac.Equal(signature, t.sign(binding, exp)) {
		return ErrMismatch
	}

	if now.Unix() > expUnix {
		return ErrExpired
	}

	return nil
}

func (t *Tokens) sign(binding, exp string) []byte {
	mac := hmac.New(sha256.New, t.secret)
	mac.Write([]byte(binding + "|" + exp))
	return mac.Sum(nil)
}
