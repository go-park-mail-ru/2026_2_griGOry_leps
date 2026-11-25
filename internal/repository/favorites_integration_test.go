package repository

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestFavoritesLifecycle(t *testing.T) {
	db := newTestPool(t)
	ctx := context.Background()
	repo := NewFavoritesRepository(db)

	_, err := db.ExecContext(ctx, `
		INSERT INTO user_account (first_name, email, phone_number, password_hash)
		VALUES
			('Продавец', 'seller@example.org', '+79990000001', 'unused'),
			('Алиса', 'alice@example.org', '+79990000002', 'unused'),
			('Тимур', 'timur@example.org', '+79990000003', 'unused');
	`)
	require.NoError(t, err)

	const (
		sellerID int32 = 1
		aliceID  int32 = 2
		timurID  int32 = 3
	)

	var listingID int32
	err = db.QueryRowContext(ctx, `
		INSERT INTO ad (user_id, title, status, price, published_at)
		VALUES ($1, 'Велосипед', 'active', 1500000, now())
		RETURNING id
	`, sellerID).Scan(&listingID)
	require.NoError(t, err)

	require.NoError(t, repo.Add(ctx, aliceID, listingID))
	require.NoError(t, repo.Add(ctx, aliceID, listingID))

	items, err := repo.List(ctx, aliceID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, listingID, items[0].ID)
	require.Equal(t, "Велосипед", items[0].Title)

	require.NoError(t, repo.Add(ctx, timurID, listingID))
	require.NoError(t, repo.Remove(ctx, timurID, listingID))
	require.NoError(t, repo.Remove(ctx, timurID, listingID))

	items, err = repo.List(ctx, timurID)
	require.NoError(t, err)
	require.Empty(t, items)

	for _, status := range []string{"closed", "sold"} {
		_, err = db.ExecContext(ctx, `UPDATE ad SET status = $1 WHERE id = $2`, status, listingID)
		require.NoError(t, err)

		require.NoError(t, repo.Add(ctx, aliceID, listingID))
		require.ErrorIs(t, repo.Add(ctx, timurID, listingID), domain.ErrListingUnavailable)

		items, err = repo.List(ctx, aliceID)
		require.NoError(t, err)
		require.Len(t, items, 1)
		require.Equal(t, listingID, items[0].ID)
		require.Equal(t, status, items[0].Status)
	}

	_, err = db.ExecContext(ctx, `DELETE FROM ad WHERE id = $1`, listingID)
	require.NoError(t, err)

	var remaining int
	err = db.QueryRowContext(ctx, `
		SELECT count(*) FROM favorites WHERE ad_id = $1
	`, listingID).Scan(&remaining)
	require.NoError(t, err)
	require.Zero(t, remaining)

	items, err = repo.List(ctx, aliceID)
	require.NoError(t, err)
	require.Empty(t, items)
	require.ErrorIs(t, repo.Add(ctx, aliceID, listingID), domain.ErrListingNotFound)
}
