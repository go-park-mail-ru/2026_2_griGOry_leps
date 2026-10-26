package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type sessionRepoFake struct {
	mu       sync.RWMutex
	sessions map[string]domain.Session
}

func newSessionRepoFake() *sessionRepoFake {
	return &sessionRepoFake{sessions: make(map[string]domain.Session)}
}

func (r *sessionRepoFake) Create(_ context.Context, id string, userID int32, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[id] = domain.Session{ID: id, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (r *sessionRepoFake) GetByID(_ context.Context, id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, domain.ErrSessionNotFound
	}
	return session, nil
}

func (r *sessionRepoFake) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, id)
	return nil
}

func (r *sessionRepoFake) DeleteOthers(_ context.Context, userID int32, keepID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, s := range r.sessions {
		if s.UserID == userID && id != keepID {
			delete(r.sessions, id)
		}
	}
	return nil
}

func (r *sessionRepoFake) DeleteAll(_ context.Context, userID int32) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, id)
		}
	}
	return nil
}

type txFake struct{}

func (txFake) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}
