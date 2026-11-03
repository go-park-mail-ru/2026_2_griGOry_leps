package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func newTestVerificationRepository(t *testing.T) (*EmailVerificationRepository, *UserRepository, int32) {
	t.Helper()
	pool := newTestPool(t)
	users := NewUserRepository(pool)
	user, err := users.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	return NewEmailVerificationRepository(pool), users, user.ID
}

func TestEmailVerification_CreateGetUse(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewEmailVerificationRepository(db)
	ctx := context.Background()
	expires := time.Now().Add(24 * time.Hour)
	mock.ExpectExec(createEmailVerificationQuery).WithArgs(hashOf("a"), int32(1), "anna@goget.pw", expires).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(ctx, hashOf("a"), 1, "anna@goget.pw", expires))
	for i, get := range []func(context.Context, string) (domain.EmailVerification, error){repo.Get, repo.GetForUpdate} {
		query := []string{getEmailVerificationQuery, getEmailVerificationForUpdateQuery}[i]
		mock.ExpectQuery(query).WithArgs(hashOf("a")).WillReturnRows(verificationRows(expires, nil))
		v, err := get(ctx, hashOf("a"))
		require.NoError(t, err)
		require.Equal(t, int32(1), v.UserID)
		require.Equal(t, "anna@goget.pw", v.Email)
		require.Equal(t, expires, v.ExpiresAt)
		require.Nil(t, v.UsedAt)
		mock.ExpectQuery(query).WithArgs(hashOf("b")).WillReturnError(sql.ErrNoRows)
		_, err = get(ctx, hashOf("b"))
		require.ErrorIs(t, err, domain.ErrInvalidVerifyToken)
	}
	mock.ExpectExec(useEmailVerificationsQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UseAll(ctx, 1))
	mock.ExpectQuery(getEmailVerificationQuery).WithArgs(hashOf("a")).WillReturnRows(verificationRows(expires, time.Now()))
	v, err := repo.Get(ctx, hashOf("a"))
	require.NoError(t, err)
	require.NotNil(t, v.UsedAt)
}

func TestEmailVerification_LastCreatedAt(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewEmailVerificationRepository(db)
	ctx := context.Background()
	mock.ExpectQuery(lastEmailVerificationQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(nil))
	last, err := repo.LastCreatedAt(ctx, 1)
	require.NoError(t, err)
	require.True(t, last.IsZero())
	now := time.Now()
	mock.ExpectExec(createEmailVerificationQuery).WithArgs(hashOf("a"), int32(1), "anna@goget.pw", now.Add(time.Hour)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Create(ctx, hashOf("a"), 1, "anna@goget.pw", now.Add(time.Hour)))
	mock.ExpectQuery(lastEmailVerificationQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"created"}).AddRow(now))
	last, err = repo.LastCreatedAt(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, now, last)
}

func TestUser_EmailVerifiedResetsOnEmailChange(t *testing.T) {
	_, users, userID := newTestVerificationRepository(t)
	ctx := context.Background()

	user, err := users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.False(t, user.EmailVerified)

	require.NoError(t, users.SetEmailVerified(ctx, userID))
	p, err := users.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.True(t, p.EmailVerified)

	name := "Аня"
	require.NoError(t, users.UpdateProfile(ctx, userID, domain.ProfileUpdate{FirstName: &name}))
	user, err = users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.True(t, user.EmailVerified)

	email := "new@goget.pw"
	require.NoError(t, users.UpdateProfile(ctx, userID, domain.ProfileUpdate{Email: &email}))
	user, err = users.GetByID(ctx, userID)
	require.NoError(t, err)
	require.False(t, user.EmailVerified)

	require.ErrorIs(t, users.SetEmailVerified(ctx, 999999), domain.ErrUserNotFound)
}

func TestUser_PendingEmail(t *testing.T) {
	db, mock := newSQLMock(t)
	users := NewUserRepository(db)
	ctx := context.Background()
	mock.ExpectExec(setEmailVerifiedQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.SetEmailVerified(ctx, 1))
	mock.ExpectExec(confirmPendingEmailQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, users.ConfirmPendingEmail(ctx, 1), domain.ErrNoPendingEmail)
	pending := "new@goget.pw"
	mock.ExpectExec(setPendingEmailQuery).WithArgs(int32(1), pending).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.SetPendingEmail(ctx, 1, &pending))
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(1)).WillReturnRows(profileMockRows(domain.Profile{User: domain.User{ID: 1, Email: "anna@goget.pw", PendingEmail: pending}}))
	p, err := users.GetProfile(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, "anna@goget.pw", p.Email)
	require.Equal(t, pending, p.PendingEmail)
	mock.ExpectExec(confirmPendingEmailQuery).WithArgs(int32(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.ConfirmPendingEmail(ctx, 1))
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnRows(userStateRows(domain.User{ID: 1, Email: pending, EmailVerified: true}))
	user, err := users.GetByID(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, pending, user.Email)
	require.Empty(t, user.PendingEmail)
	require.True(t, user.EmailVerified)
	mock.ExpectExec(setPendingEmailQuery).WithArgs(int32(1), pending).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.SetPendingEmail(ctx, 1, &pending))
	mock.ExpectExec(setPendingEmailQuery).WithArgs(int32(1), nil).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.SetPendingEmail(ctx, 1, nil))
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnRows(userStateRows(domain.User{ID: 1, Email: pending, EmailVerified: true}))
	user, err = users.GetByID(ctx, 1)
	require.NoError(t, err)
	require.Empty(t, user.PendingEmail)
}

func TestUser_ConfirmPendingEmailTaken(t *testing.T) {
	db, mock := newSQLMock(t)
	users := NewUserRepository(db)
	pending := "boris@goget.pw"
	mock.ExpectExec(setPendingEmailQuery).WithArgs(int32(1), pending).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, users.SetPendingEmail(context.Background(), 1, &pending))
	mock.ExpectExec(confirmPendingEmailQuery).WithArgs(int32(1)).WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "user_account_email_key"})
	require.ErrorIs(t, users.ConfirmPendingEmail(context.Background(), 1), domain.ErrEmailTaken)
}

func verificationRows(expires time.Time, used any) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"user_id", "email", "expires_at", "used_at"}).AddRow(1, "anna@goget.pw", expires, used)
}
