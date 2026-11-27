package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestSellerUsecase_Listings_Defaults(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().ListAds(gomock.Any(), domain.SellerAdFilter{SellerID: 5, Sort: domain.AdSortNewest, Limit: maxAdsLimit}).Return(domain.SellerAdPage{}, nil)

	_, err := NewSellerUsecase(repo, newFakeDirectory()).Listings(context.Background(), domain.SellerAdFilter{SellerID: 5, Limit: 500})
	require.NoError(t, err)

}

func TestSellerUsecase_Listings_InvalidSort(t *testing.T) {
	_, err := NewSellerUsecase(NewMockSellerRepository(gomock.NewController(t)), newFakeDirectory()).Listings(context.Background(), domain.SellerAdFilter{Sort: "rating_desc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Reviews(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().ListReviews(gomock.Any(), domain.ReviewFilter{SellerID: 5, Limit: 10, WithPhotos: true, Sort: domain.ReviewSortNewest}).Return(domain.ReviewPage{}, nil)
	uc := NewSellerUsecase(repo, newFakeDirectory())

	_, err := uc.Reviews(context.Background(), domain.ReviewFilter{SellerID: 5, Limit: 10, WithPhotos: true})
	require.NoError(t, err)

	_, err = uc.Reviews(context.Background(), domain.ReviewFilter{Sort: "price_asc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Report(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().CreateReport(gomock.Any(), domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonRude, Comment: "Хамил в переписке"}).Return(nil)

	err := NewSellerUsecase(repo, newFakeDirectory()).Report(context.Background(), domain.SellerReport{
		SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonRude, Comment: "  Хамил в переписке ",
	})
	require.NoError(t, err)
}

func TestSellerUsecase_Report_Errors(t *testing.T) {
	tests := []struct {
		name   string
		report domain.SellerReport
		want   error
	}{
		{"unknown reason", domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: "spam"}, domain.ErrInvalidReportReason},
		{"long comment", domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther,
			Comment: strings.Repeat("я", maxReportCommentLength+1)}, domain.ErrReportCommentTooLong},
		{"self", domain.SellerReport{SellerID: 7, ReporterID: 7, Reason: domain.ReportReasonFraud}, domain.ErrSelfReport},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockSellerRepository(gomock.NewController(t))
			require.ErrorIs(t, NewSellerUsecase(repo, newFakeDirectory()).Report(context.Background(), tt.report), tt.want)
		})
	}
}

func TestSellerUsecase_Report_CommentLimitCountsRunes(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().CreateReport(gomock.Any(), domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther, Comment: strings.Repeat("я", maxReportCommentLength)}).Return(nil)
	err := NewSellerUsecase(repo, newFakeDirectory()).Report(context.Background(), domain.SellerReport{
		SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther, Comment: strings.Repeat("я", maxReportCommentLength),
	})
	require.NoError(t, err)
}

func sellerCard() domain.UserCard {
	return domain.UserCard{ID: 5, FirstName: "Анна", Avatar: "a.png", City: "Казань", PhoneVerified: true, CreatedAt: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
}

func TestSellerUsecase_Profile_MergesDirectoryAndStats(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	card := sellerCard()
	repo.EXPECT().Get(gomock.Any(), int32(5)).Return(domain.SellerProfile{
		Seller: domain.Seller{ID: 5, Rating: 4.5, ReviewsCount: 2}, CompletedSalesCount: 1, ActiveListingsCount: 3,
	}, nil)

	got, err := NewSellerUsecase(repo, newFakeDirectory(card)).Profile(context.Background(), 5)

	require.NoError(t, err)
	require.Equal(t, domain.SellerProfile{
		Seller:   domain.Seller{ID: 5, FirstName: "Анна", Avatar: "a.png", Rating: 4.5, ReviewsCount: 2, CreatedAt: card.CreatedAt},
		Location: "Казань", PhoneVerified: true, CompletedSalesCount: 1, ActiveListingsCount: 3,
	}, got)
}

func TestSellerUsecase_UnknownSeller(t *testing.T) {
	dir := newFakeDirectory()
	dir.missing[999] = true
	uc := NewSellerUsecase(NewMockSellerRepository(gomock.NewController(t)), dir)
	ctx := context.Background()

	_, err := uc.Profile(ctx, 999)
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
	_, err = uc.Listings(ctx, domain.SellerAdFilter{SellerID: 999})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
	err = uc.Report(ctx, domain.SellerReport{SellerID: 999, ReporterID: 7, Reason: domain.ReportReasonRude})
	require.ErrorIs(t, err, domain.ErrSellerNotFound)
}

func TestSellerUsecase_Reviews_FillsAuthorsInOneCall(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	dir := newFakeDirectory(domain.UserCard{ID: 2, FirstName: "Олег", Avatar: "o.png"}, domain.UserCard{ID: 3, FirstName: "Мария"})
	repo.EXPECT().ListReviews(gomock.Any(), gomock.Any()).Return(domain.ReviewPage{Items: []domain.Review{
		{ID: 1, Author: domain.ReviewAuthor{ID: 2}}, {ID: 2, Author: domain.ReviewAuthor{ID: 3}}, {ID: 3, Author: domain.ReviewAuthor{ID: 2}},
	}}, nil)

	page, err := NewSellerUsecase(repo, dir).Reviews(context.Background(), domain.ReviewFilter{SellerID: 5, Limit: 10})

	require.NoError(t, err)
	require.Equal(t, domain.ReviewAuthor{ID: 2, FirstName: "Олег", Avatar: "o.png"}, page.Items[0].Author)
	require.Equal(t, "Мария", page.Items[1].Author.FirstName)
	require.Equal(t, "Олег", page.Items[2].Author.FirstName)
	require.Equal(t, [][]int32{{5, 2, 3}}, dir.calls)
}

func TestSellerUsecase_Reviews_MissingAuthorOrSeller(t *testing.T) {
	for name, tc := range map[string]struct {
		missing int32
		want    error
	}{"author": {2, domain.ErrUserMissing}, "seller": {5, domain.ErrSellerNotFound}} {
		t.Run(name, func(t *testing.T) {
			repo := NewMockSellerRepository(gomock.NewController(t))
			dir := newFakeDirectory()
			dir.missing[tc.missing] = true
			repo.EXPECT().ListReviews(gomock.Any(), gomock.Any()).Return(domain.ReviewPage{Items: []domain.Review{{ID: 1, Author: domain.ReviewAuthor{ID: 2}}}}, nil)

			_, err := NewSellerUsecase(repo, dir).Reviews(context.Background(), domain.ReviewFilter{SellerID: 5, Limit: 10})

			require.ErrorIs(t, err, tc.want)
		})
	}
}
