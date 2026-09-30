package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]domain.Session
}

func NewSessionRepository() *SessionRepository {
	return &SessionRepository{sessions: make(map[string]domain.Session)}
}

func (r *SessionRepository) Create(_ context.Context, id string, userID int32, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[id] = domain.Session{ID: id, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (r *SessionRepository) GetByID(_ context.Context, id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrSessionNotFound
	}
	return session, nil
}

func (r *SessionRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, id)
	return nil
}
