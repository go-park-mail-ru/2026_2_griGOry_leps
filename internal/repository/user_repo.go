package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

var (
	ErrUserNotFound = errors.New("user not found")
	ErrUserExists   = errors.New("user already exists")
	ErrPhoneExists  = errors.New("phone already registered")
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = "id, email, password_hash, firstname, nickname, phonenumber, created_at"

func scanUser(row pgx.Row, user *domain.User) error {
	return row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.FirstName, &user.Nickname, &user.Phone, &user.CreatedAt)
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash, firstName, nickname, phone string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var user domain.User

	row := r.db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, firstname, nickname, phonenumber)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+userColumns,
		email, passwordHash, firstName, nickname, phone,
	)

	if err := scanUser(row, &user); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			if pgErr.ConstraintName == "users_phonenumber_key" {
				return domain.User{}, ErrPhoneExists
			}
			return domain.User{}, ErrUserExists
		}
		return domain.User{}, err
	}

	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var user domain.User

	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE email = $1`, email)

	if err := scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return user, nil
}

func (r *UserRepository) GetByPhone(ctx context.Context, phone string) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var user domain.User

	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE phonenumber = $1`, phone)

	if err := scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int32) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, dbTimeout)
	defer cancel()

	var user domain.User

	row := r.db.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)

	if err := scanUser(row, &user); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, err
	}

	return user, nil
}
