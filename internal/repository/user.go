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
	user, err := scanUser(conn(ctx, r.db).QueryRow(ctx, createUserQuery, email, passwordHash, firstName, phone))
	if err != nil {
		if domainErr := constraintError(err); domainErr != nil {
			return domain.User{}, domainErr
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func constraintError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}

	switch pgErr.ConstraintName {
	case "user_account_email_key":
		return domain.ErrEmailTaken
	case "user_account_phone_number_key":
		return domain.ErrPhoneTaken
	case "user_account_city_id_fkey":
		return domain.ErrInvalidCity
	}
	return nil
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
	user, err := scanUser(conn(ctx, r.db).QueryRow(ctx, query, arg))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetProfile(ctx context.Context, id int32) (domain.Profile, error) {
	var (
		p        domain.Profile
		cityID   *int32
		cityName *string
	)
	err := conn(ctx, r.db).QueryRow(ctx, getProfileQuery, id).Scan(
		&p.ID, &p.FirstName, &p.Email, &p.Phone, &p.CreatedAt,
		&p.PhoneVerified, &cityID, &cityName, &p.About, &p.Avatar,
		&p.Rating, &p.ReviewsCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Profile{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.Profile{}, fmt.Errorf("get profile: %w", err)
	}

	if cityID != nil && cityName != nil {
		p.City = &domain.City{ID: *cityID, Name: *cityName}
	}
	return p, nil
}

func (r *UserRepository) UpdateProfile(ctx context.Context, id int32, upd domain.ProfileUpdate) error {
	tag, err := conn(ctx, r.db).Exec(ctx, updateProfileQuery,
		id, upd.FirstName, upd.Phone, upd.Email, upd.CityID, upd.About)
	if err != nil {
		if domainErr := constraintError(err); domainErr != nil {
			return domainErr
		}
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, id int32, passwordHash string) error {
	tag, err := conn(ctx, r.db).Exec(ctx, updatePasswordQuery, id, passwordHash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) GetAvatarForUpdate(ctx context.Context, id int32) (string, error) {
	var avatar string
	err := conn(ctx, r.db).QueryRow(ctx, getAvatarForUpdateQuery, id).Scan(&avatar)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrUserNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get avatar: %w", err)
	}
	return avatar, nil
}

func (r *UserRepository) UpdateAvatar(ctx context.Context, id int32, avatar string) error {
	tag, err := conn(ctx, r.db).Exec(ctx, updateAvatarQuery, id, avatar)
	if err != nil {
		return fmt.Errorf("update avatar: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
