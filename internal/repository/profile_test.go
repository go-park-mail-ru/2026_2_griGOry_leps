package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func newTestProfileRepository(t *testing.T) (*UserRepository, *sql.DB, int32) {
	t.Helper()
	pool := newTestPool(t)
	repo := NewUserRepository(pool)
	user, err := repo.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	return repo, pool, user.ID
}

func cityID(t *testing.T, pool *sql.DB, name string) int32 {
	t.Helper()
	var id int32
	require.NoError(t, pool.QueryRowContext(context.Background(), "SELECT city_id FROM city WHERE name = $1", name).Scan(&id))
	return id
}

func TestProfile_GetDefaults(t *testing.T) {
	db, mock := newSQLMock(t)
	want := domain.Profile{User: domain.User{ID: 1, FirstName: "Anna", Phone: "+79990000001", CreatedAt: time.Now()}}
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(1)).WillReturnRows(profileMockRows(want))
	actual, err := NewUserRepository(db).GetProfile(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, want, actual)
}

func TestProfile_GetNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(999)).WillReturnError(sql.ErrNoRows)
	_, err := NewUserRepository(db).GetProfile(context.Background(), 999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestProfile_RatingFromReviewsOnOwnAds(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(1)).WillReturnRows(profileMockRows(domain.Profile{User: domain.User{ID: 1}, Rating: 4.5, ReviewsCount: 2}))
	p, err := repo.GetProfile(context.Background(), 1)
	require.NoError(t, err)
	require.InDelta(t, 4.5, p.Rating, 0.001)
	require.Equal(t, 2, p.ReviewsCount)
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(2)).WillReturnRows(profileMockRows(domain.Profile{User: domain.User{ID: 2}}))
	p, err = repo.GetProfile(context.Background(), 2)
	require.NoError(t, err)
	require.Zero(t, p.ReviewsCount)
}

func TestProfile_UpdatePartial(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	city, about := int32(7), "Selling things"
	mock.ExpectExec(updateProfileQuery).WithArgs(int32(1), nil, nil, nil, city, about).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateProfile(context.Background(), 1, domain.ProfileUpdate{CityID: &city, About: &about}))
	want := domain.Profile{User: domain.User{ID: 1, FirstName: "Anna", Email: "anna@goget.pw"}, City: &domain.City{ID: city, Name: "Moscow"}, About: about}
	mock.ExpectQuery(getProfileQuery).WithArgs(int32(1)).WillReturnRows(profileMockRows(want))
	p, err := repo.GetProfile(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, want, p)
}

func TestProfile_UpdatePhoneResetsVerification(t *testing.T) {
	repo, pool, userID := newTestProfileRepository(t)
	ctx := context.Background()
	_, err := pool.ExecContext(ctx, "UPDATE user_account SET phone_verified = true WHERE user_id = $1", userID)
	require.NoError(t, err)

	same := "+79990000001"
	require.NoError(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{Phone: &same}))
	p, err := repo.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.True(t, p.PhoneVerified)

	other := "+79990000009"
	require.NoError(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{Phone: &other}))
	p, err = repo.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.False(t, p.PhoneVerified)
	require.Equal(t, other, p.Phone)
}

func TestProfile_UpdateErrors(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	email, phone, city := "other@goget.pw", "+79990000002", int32(100500)
	mock.ExpectExec(updateProfileQuery).WithArgs(int32(1), nil, nil, email, nil, nil).WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "user_account_email_key"})
	require.ErrorIs(t, repo.UpdateProfile(context.Background(), 1, domain.ProfileUpdate{Email: &email}), domain.ErrEmailTaken)
	mock.ExpectExec(updateProfileQuery).WithArgs(int32(1), nil, phone, nil, nil, nil).WillReturnError(&pgconn.PgError{Code: "23505", ConstraintName: "user_account_phone_number_key"})
	require.ErrorIs(t, repo.UpdateProfile(context.Background(), 1, domain.ProfileUpdate{Phone: &phone}), domain.ErrPhoneTaken)
	mock.ExpectExec(updateProfileQuery).WithArgs(int32(1), nil, nil, nil, city, nil).WillReturnError(&pgconn.PgError{Code: "23503", ConstraintName: "user_account_city_id_fkey"})
	require.ErrorIs(t, repo.UpdateProfile(context.Background(), 1, domain.ProfileUpdate{CityID: &city}), domain.ErrInvalidCity)
	mock.ExpectExec(updateProfileQuery).WithArgs(int32(999), nil, nil, nil, nil, nil).WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.UpdateProfile(context.Background(), 999, domain.ProfileUpdate{}), domain.ErrUserNotFound)
}

func TestProfile_UpdatePassword(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	mock.ExpectExec(updatePasswordQuery).WithArgs(int32(1), "new-hash").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdatePassword(context.Background(), 1, "new-hash"))
	mock.ExpectQuery(getUserByIDQuery).WithArgs(int32(1)).WillReturnRows(userStateRows(domain.User{ID: 1, PasswordHash: "new-hash"}))
	user, err := repo.GetByID(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "new-hash", user.PasswordHash)
	mock.ExpectExec(updatePasswordQuery).WithArgs(int32(999), "x").WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.UpdatePassword(context.Background(), 999, "x"), domain.ErrUserNotFound)
}

func TestProfile_UpdateAvatar(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := NewUserRepository(db)
	mock.ExpectQuery(getAvatarForUpdateQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"avatar"}).AddRow(""))
	old, err := repo.GetAvatarForUpdate(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, old)
	mock.ExpectExec(updateAvatarQuery).WithArgs(int32(1), "first.png").WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateAvatar(context.Background(), 1, "first.png"))
	mock.ExpectQuery(getAvatarForUpdateQuery).WithArgs(int32(1)).WillReturnRows(sqlmock.NewRows([]string{"avatar"}).AddRow("first.png"))
	old, err = repo.GetAvatarForUpdate(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, "first.png", old)
	mock.ExpectQuery(getAvatarForUpdateQuery).WithArgs(int32(999)).WillReturnError(sql.ErrNoRows)
	_, err = repo.GetAvatarForUpdate(context.Background(), 999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
	mock.ExpectExec(updateAvatarQuery).WithArgs(int32(999), "x.png").WillReturnResult(sqlmock.NewResult(0, 0))
	require.ErrorIs(t, repo.UpdateAvatar(context.Background(), 999, "x.png"), domain.ErrUserNotFound)
}

func TestProfile_ConcurrentAvatarUploadsSeeEachOther(t *testing.T) {
	repo, pool, userID := newTestProfileRepository(t)
	tx := NewTransactor(pool)
	ctx := context.Background()

	locked := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- tx.WithinTx(ctx, func(ctx context.Context) error {
			if _, err := repo.GetAvatarForUpdate(ctx, userID); err != nil {
				return err
			}
			close(locked)
			<-release
			return repo.UpdateAvatar(ctx, userID, "first.png")
		})
	}()

	select {
	case <-locked:
	case err := <-firstDone:
		t.Fatal(err)
	}

	secondOld := make(chan string, 1)
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- tx.WithinTx(ctx, func(ctx context.Context) error {
			old, err := repo.GetAvatarForUpdate(ctx, userID)
			if err != nil {
				return err
			}
			secondOld <- old
			return repo.UpdateAvatar(ctx, userID, "second.png")
		})
	}()

	select {
	case <-secondOld:
		t.Fatal("second upload read the avatar while the first one held the lock")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-firstDone)
	require.Equal(t, "first.png", <-secondOld)
	require.NoError(t, <-secondDone)
}

func TestProfile_PasswordHashLockWaitsForChange(t *testing.T) {
	repo, pool, userID := newTestProfileRepository(t)
	tx := NewTransactor(pool)
	ctx := context.Background()

	_, err := repo.GetPasswordHashForUpdate(ctx, 999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)

	locked := make(chan struct{})
	release := make(chan struct{})
	changeDone := make(chan error, 1)
	go func() {
		changeDone <- tx.WithinTx(ctx, func(ctx context.Context) error {
			hash, err := repo.GetPasswordHashForUpdate(ctx, userID)
			if err != nil {
				return err
			}
			if hash != "hash" {
				return fmt.Errorf("unexpected hash %q", hash)
			}
			close(locked)
			<-release
			return repo.UpdatePassword(ctx, userID, "new-hash")
		})
	}()

	select {
	case <-locked:
	case err := <-changeDone:
		t.Fatal(err)
	}

	loginHash := make(chan string, 1)
	loginDone := make(chan error, 1)
	go func() {
		loginDone <- tx.WithinTx(ctx, func(ctx context.Context) error {
			hash, err := repo.GetPasswordHashForUpdate(ctx, userID)
			loginHash <- hash
			return err
		})
	}()

	select {
	case <-loginHash:
		t.Fatal("login read the password hash while it was being changed")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	require.NoError(t, <-changeDone)
	require.Equal(t, "new-hash", <-loginHash)
	require.NoError(t, <-loginDone)
}

func profileMockRows(p domain.Profile) *sqlmock.Rows {
	var cityID, cityName any
	if p.City != nil {
		cityID, cityName = p.City.ID, p.City.Name
	}
	return sqlmock.NewRows([]string{"id", "name", "email", "phone", "created", "email_verified", "pending_email", "phone_verified", "city_id", "city_name", "about", "avatar", "rating", "reviews"}).
		AddRow(p.ID, p.FirstName, p.Email, p.Phone, p.CreatedAt, p.EmailVerified, p.PendingEmail, p.PhoneVerified, cityID, cityName, p.About, p.Avatar, p.Rating, p.ReviewsCount)
}
func userStateRows(u domain.User) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "name", "email", "phone", "hash", "email_verified", "pending_email", "created"}).
		AddRow(u.ID, u.FirstName, u.Email, u.Phone, u.PasswordHash, u.EmailVerified, u.PendingEmail, u.CreatedAt)
}
