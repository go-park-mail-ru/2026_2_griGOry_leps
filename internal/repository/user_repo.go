package repository

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrPhoneExists  = errors.New("phone already registered")
)

type UserRepository struct {
	mu      sync.RWMutex
	lastID  int32
	users   map[int32]domain.User
	byEmail map[string]int32
	byPhone map[string]int32
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		users:   make(map[int32]domain.User),
		byEmail: make(map[string]int32),
		byPhone: make(map[string]int32),
	}
}

func (r *UserRepository) Create(_ context.Context, email, passwordHash, firstName, nickname, phone string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byEmail[email]; ok {
		return domain.User{}, ErrUserExists
	}
	if _, ok := r.byPhone[phone]; ok {
		return domain.User{}, ErrPhoneExists
	}

	r.lastID++
	user := domain.User{
		ID:           r.lastID,
		Email:        email,
		PasswordHash: passwordHash,
		FirstName:    firstName,
		Nickname:     nickname,
		Phone:        phone,
		CreatedAt:    time.Now(),
	}

	r.users[user.ID] = user
	r.byEmail[email] = user.ID
	r.byPhone[phone] = user.ID

	return user, nil
}

func (r *UserRepository) GetByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	return r.users[id], nil
}

func (r *UserRepository) GetByPhone(_ context.Context, phone string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byPhone[phone]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	return r.users[id], nil
}

func (r *UserRepository) GetByID(_ context.Context, id int32) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	return user, nil
}
