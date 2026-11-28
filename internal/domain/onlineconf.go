package domain

import (
	"encoding/json"
	"errors"
)

type OnlineconfEntry struct {
	ID    int             `json:"id"`
	Scope string          `json:"scope"`
	Name  string          `json:"name"`
	Kind  string          `json:"kind"`
	Value json.RawMessage `json:"value"`
}

type OnlineconfPatch struct {
	Scope *string
	Name  *string
	Value json.RawMessage
}

type OnlineconfFilter struct {
	Name  string
	Scope *string
	Kind  *string
}

var (
	ErrInvalidOnlineconf = errors.New("invalid onlineconf entry or filter")
	ErrOnlineconfMissing = errors.New("onlineconf entry not found")
	ErrOnlineconfExists  = errors.New("onlineconf name already exists in scope")
)
