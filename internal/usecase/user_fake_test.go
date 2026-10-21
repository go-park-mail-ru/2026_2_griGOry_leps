package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type userRepoFake struct {
	mu      sync.RWMutex
	lastID  int32
	users   map[int32]domain.User
	extra   map[int32]domain.Profile
	byEmail map[string]int32
	byPhone map[string]int32
	cities  map[int32]string
}

func newUserRepoFake() *userRepoFake {
	return &userRepoFake{
		users:   make(map[int32]domain.User),
		extra:   make(map[int32]domain.Profile),
		byEmail: make(map[string]int32),
		byPhone: make(map[string]int32),
		cities:  map[int32]string{1: "Москва", 2: "Химки"},
	}
}

func (r *userRepoFake) Create(_ context.Context, email, passwordHash, firstName, phone string) (domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byEmail[email]; ok {
		return domain.User{}, domain.ErrEmailTaken
	}
	if _, ok := r.byPhone[phone]; ok {
		return domain.User{}, domain.ErrPhoneTaken
	}

	r.lastID++
	user := domain.User{
		ID:           r.lastID,
		Email:        email,
		PasswordHash: passwordHash,
		FirstName:    firstName,
		Phone:        phone,
		CreatedAt:    time.Now(),
	}

	r.users[user.ID] = user
	r.byEmail[email] = user.ID
	r.byPhone[phone] = user.ID

	return user, nil
}

func (r *userRepoFake) GetByEmail(_ context.Context, email string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byEmail[email]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return r.users[id], nil
}

func (r *userRepoFake) GetByPhone(_ context.Context, phone string) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byPhone[phone]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return r.users[id], nil
}

func (r *userRepoFake) GetByID(_ context.Context, id int32) (domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return domain.User{}, domain.ErrUserNotFound
	}
	return user, nil
}

func (r *userRepoFake) GetProfile(_ context.Context, id int32) (domain.Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return domain.Profile{}, domain.ErrUserNotFound
	}
	p := r.extra[id]
	p.User = user
	p.PasswordHash = ""
	return p, nil
}

func (r *userRepoFake) UpdateProfile(_ context.Context, id int32, upd domain.ProfileUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	if upd.Email != nil {
		if other, ok := r.byEmail[*upd.Email]; ok && other != id {
			return domain.ErrEmailTaken
		}
	}
	if upd.Phone != nil {
		if other, ok := r.byPhone[*upd.Phone]; ok && other != id {
			return domain.ErrPhoneTaken
		}
	}

	p := r.extra[id]
	if upd.CityID != nil {
		name, ok := r.cities[*upd.CityID]
		if !ok {
			return domain.ErrInvalidCity
		}
		p.City = &domain.City{ID: *upd.CityID, Name: name}
	}
	if upd.About != nil {
		p.About = *upd.About
	}
	if upd.FirstName != nil {
		user.FirstName = *upd.FirstName
	}
	if upd.Email != nil {
		delete(r.byEmail, user.Email)
		user.Email = *upd.Email
		r.byEmail[user.Email] = id
	}
	if upd.Phone != nil {
		delete(r.byPhone, user.Phone)
		user.Phone = *upd.Phone
		r.byPhone[user.Phone] = id
	}

	r.users[id] = user
	r.extra[id] = p
	return nil
}

func (r *userRepoFake) UpdatePassword(_ context.Context, id int32, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, ok := r.users[id]
	if !ok {
		return domain.ErrUserNotFound
	}
	user.PasswordHash = passwordHash
	r.users[id] = user
	return nil
}

func (r *userRepoFake) GetAvatarForUpdate(_ context.Context, id int32) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.users[id]; !ok {
		return "", domain.ErrUserNotFound
	}
	return r.extra[id].Avatar, nil
}

func (r *userRepoFake) UpdateAvatar(_ context.Context, id int32, avatar string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.users[id]; !ok {
		return domain.ErrUserNotFound
	}
	p := r.extra[id]
	p.Avatar = avatar
	r.extra[id] = p
	return nil
}
