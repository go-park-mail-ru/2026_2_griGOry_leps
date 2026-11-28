package domain

import (
	"encoding/json"
	"errors"
	"time"
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
	ErrInvalidUserID     = errors.New("user_id must be positive")
	ErrInvalidOnlineconf = errors.New("invalid onlineconf entry or filter")
	ErrOnlineconfMissing = errors.New("onlineconf entry not found")
	ErrOnlineconfExists  = errors.New("onlineconf name already exists in scope")
)

type OnlineconfChange struct {
	ID        int64            `json:"id"`
	EntryID   int              `json:"entry_id"`
	ActorID   int32            `json:"actor_id"`
	Operation string           `json:"operation"`
	ChangedAt time.Time        `json:"changed_at"`
	Before    *OnlineconfEntry `json:"before"`
	After     OnlineconfEntry  `json:"after"`
}
