//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func newTestProfileRepository(t *testing.T) (*UserRepository, *pgxpool.Pool, int32) {
	t.Helper()
	pool := newTestPool(t)
	repo := NewUserRepository(pool)
	user, err := repo.Create(context.Background(), "anna@goget.pw", "hash", "Анна", "+79990000001")
	require.NoError(t, err)
	return repo, pool, user.ID
}

func cityID(t *testing.T, pool *pgxpool.Pool, name string) int32 {
	t.Helper()
	var id int32
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT city_id FROM city WHERE name = $1", name).Scan(&id))
	return id
}

func TestProfile_GetDefaults(t *testing.T) {
	repo, _, userID := newTestProfileRepository(t)

	p, err := repo.GetProfile(context.Background(), userID)
	require.NoError(t, err)
	require.Equal(t, "Анна", p.FirstName)
	require.Equal(t, "+79990000001", p.Phone)
	require.Nil(t, p.City)
	require.Empty(t, p.About)
	require.Empty(t, p.Avatar)
	require.Zero(t, p.Rating)
	require.Zero(t, p.ReviewsCount)
	require.False(t, p.PhoneVerified)
	require.NotZero(t, p.CreatedAt)
}

func TestProfile_GetNotFound(t *testing.T) {
	repo := NewUserRepository(newTestPool(t))

	_, err := repo.GetProfile(context.Background(), 999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestProfile_RatingFromReviewsOnOwnAds(t *testing.T) {
	repo, pool, sellerID := newTestProfileRepository(t)
	ctx := context.Background()
	buyer, err := repo.Create(ctx, "buyer@goget.pw", "hash", "Олег", "+79990000002")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		WITH ads AS (
			INSERT INTO ad (title, price, user_id) VALUES ('Велосипед', 100, $1), ('Самокат', 200, $1)
			RETURNING id
		)
		INSERT INTO review (user_id, ad_id, rating)
		SELECT $2, id, CASE WHEN row_number() OVER (ORDER BY id) = 1 THEN 5 ELSE 4 END FROM ads`,
		sellerID, buyer.ID)
	require.NoError(t, err)

	p, err := repo.GetProfile(ctx, sellerID)
	require.NoError(t, err)
	require.InDelta(t, 4.5, p.Rating, 0.001)
	require.Equal(t, 2, p.ReviewsCount)

	p, err = repo.GetProfile(ctx, buyer.ID)
	require.NoError(t, err)
	require.Zero(t, p.ReviewsCount)
}

func TestProfile_UpdatePartial(t *testing.T) {
	repo, pool, userID := newTestProfileRepository(t)
	ctx := context.Background()
	moscow := cityID(t, pool, "Москва")
	about := "Продаю вещи"

	require.NoError(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{CityID: &moscow, About: &about}))

	p, err := repo.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, &domain.City{ID: moscow, Name: "Москва"}, p.City)
	require.Equal(t, about, p.About)
	require.Equal(t, "Анна", p.FirstName)
	require.Equal(t, "anna@goget.pw", p.Email)
}

func TestProfile_UpdatePhoneResetsVerification(t *testing.T) {
	repo, pool, userID := newTestProfileRepository(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "UPDATE user_account SET phone_verified = true WHERE user_id = $1", userID)
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
	repo, _, userID := newTestProfileRepository(t)
	ctx := context.Background()
	_, err := repo.Create(ctx, "other@goget.pw", "hash", "Олег", "+79990000002")
	require.NoError(t, err)

	email := "other@goget.pw"
	phone := "+79990000002"
	city := int32(100500)

	require.ErrorIs(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{Email: &email}), domain.ErrEmailTaken)
	require.ErrorIs(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{Phone: &phone}), domain.ErrPhoneTaken)
	require.ErrorIs(t, repo.UpdateProfile(ctx, userID, domain.ProfileUpdate{CityID: &city}), domain.ErrInvalidCity)
	require.ErrorIs(t, repo.UpdateProfile(ctx, 999, domain.ProfileUpdate{}), domain.ErrUserNotFound)
}

func TestProfile_UpdatePassword(t *testing.T) {
	repo, _, userID := newTestProfileRepository(t)
	ctx := context.Background()

	require.NoError(t, repo.UpdatePassword(ctx, userID, "new-hash"))

	user, err := repo.GetByID(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "new-hash", user.PasswordHash)
	require.ErrorIs(t, repo.UpdatePassword(ctx, 999, "x"), domain.ErrUserNotFound)
}

func TestProfile_UpdateAvatarReturnsOld(t *testing.T) {
	repo, _, userID := newTestProfileRepository(t)
	ctx := context.Background()

	old, err := repo.UpdateAvatar(ctx, userID, "first.png")
	require.NoError(t, err)
	require.Empty(t, old)

	old, err = repo.UpdateAvatar(ctx, userID, "second.jpg")
	require.NoError(t, err)
	require.Equal(t, "first.png", old)

	p, err := repo.GetProfile(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, "second.jpg", p.Avatar)

	_, err = repo.UpdateAvatar(ctx, 999, "x.png")
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}
