package logger

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

const RequestIDHeader = "X-Request-Id"

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func NormalizeRequestID(id string) string {
	if validRequestID.MatchString(id) {
		return id
	}
	var bytes [16]byte
	_, _ = rand.Read(bytes[:])
	return hex.EncodeToString(bytes[:])
}
