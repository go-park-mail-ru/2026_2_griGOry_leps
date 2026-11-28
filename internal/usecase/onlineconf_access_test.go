package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestOnlineConfAccess_CheckAdmin(t *testing.T) {
	dbErr := errors.New("db down")
	for _, tc := range []struct {
		name             string
		admin            bool
		repoErr, wantErr error
	}{
		{"admin", true, nil, nil},
		{"regular", false, nil, domain.ErrForbidden},
		{"db error", false, dbErr, dbErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMockOnlineConfAdminRepository(gomock.NewController(t))
			repo.EXPECT().IsAdmin(gomock.Any(), int32(7)).Return(tc.admin, tc.repoErr)
			err := NewOnlineConfAccessUsecase(repo).CheckAdmin(context.Background(), 7)
			require.ErrorIs(t, err, tc.wantErr)
			if tc.repoErr != nil {
				require.NotErrorIs(t, err, domain.ErrForbidden)
			}
		})
	}
}

func TestOnlineConfAccess_CheckAdmin_NonPositiveID(t *testing.T) {
	repo := NewMockOnlineConfAdminRepository(gomock.NewController(t))
	for _, id := range []int32{0, -1} {
		require.ErrorIs(t, NewOnlineConfAccessUsecase(repo).CheckAdmin(context.Background(), id), domain.ErrForbidden)
	}
}

func TestOnlineConfAccess_GrantAdmin(t *testing.T) {
	dbErr := errors.New("db down")
	granter := int32(1)
	for _, tc := range []struct {
		name    string
		created bool
		err     error
		by      *int32
	}{
		{"granted", true, nil, nil},
		{"already admin", false, nil, &granter},
		{"db error", false, dbErr, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewMockOnlineConfAdminRepository(gomock.NewController(t))
			repo.EXPECT().Grant(gomock.Any(), int32(7), tc.by).Return(tc.created, tc.err)
			created, err := NewOnlineConfAccessUsecase(repo).GrantAdmin(context.Background(), 7, tc.by)
			require.Equal(t, tc.created, created)
			require.ErrorIs(t, err, tc.err)
		})
	}
	for _, id := range []int32{0, -3} {
		repo := NewMockOnlineConfAdminRepository(gomock.NewController(t))
		created, err := NewOnlineConfAccessUsecase(repo).GrantAdmin(context.Background(), id, nil)
		require.False(t, created)
		require.ErrorIs(t, err, domain.ErrInvalidUserID)
	}
}
