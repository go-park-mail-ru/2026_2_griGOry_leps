package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type UserRepository struct {
	db DB
}

func NewUserRepository(db DB) *UserRepository {
	return &UserRepository{db: db}
}

func scanUser(row pgx.Row) (domain.User, error) {
	var user domain.User
	err := row.Scan(&user.ID, &user.FirstName, &user.Email, &user.Phone, &user.PasswordHash, &user.CreatedAt)
	return user, err
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash, firstName, phone string) (domain.User, error) {
	var user domain.User
	user, err := scanUser(r.db.QueryRow(ctx, createUserQuery, email, passwordHash, firstName, phone))

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "user_account_email_key":
			return domain.User{}, domain.ErrEmailTaken
		case "user_account_phone_number_key":
			return domain.User{}, domain.ErrPhoneTaken
		}
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.get(ctx, getUserByEmailQuery, email)
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (domain.User, error) {
	return r.get(ctx, getUserByPhoneQuery, phone)
}

func (r *UserRepository) GetByID(ctx context.Context, id int32) (domain.User, error) {
	return r.get(ctx, getUserByIDQuery, id)
}

func (r *UserRepository) get(ctx context.Context, query string, arg any) (domain.User, error) {
	var user domain.User
	user, err := scanUser(r.db.QueryRow(ctx, query, arg))

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, err
}
