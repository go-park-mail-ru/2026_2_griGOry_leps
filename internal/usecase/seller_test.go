package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type sellerRepoStub struct {
	adFilter     domain.SellerAdFilter
	reviewFilter domain.ReviewFilter
	report       *domain.SellerReport
}

func (s *sellerRepoStub) Get(_ context.Context, id int32) (domain.SellerProfile, error) {
	return domain.SellerProfile{SellerShort: domain.SellerShort{ID: id}}, nil
}

func (s *sellerRepoStub) ListAds(_ context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error) {
	s.adFilter = f
	return domain.SellerAdPage{AdPage: domain.AdPage{Limit: f.Limit}}, nil
}

func (s *sellerRepoStub) ListReviews(_ context.Context, f domain.ReviewFilter) (domain.ReviewPage, error) {
	s.reviewFilter = f
	return domain.ReviewPage{Limit: f.Limit}, nil
}

func (s *sellerRepoStub) CreateReport(_ context.Context, report domain.SellerReport) error {
	s.report = &report
	return nil
}

func TestSellerUsecase_Listings_Defaults(t *testing.T) {
	repo := &sellerRepoStub{}

	_, err := NewSellerUsecase(repo).Listings(context.Background(), domain.SellerAdFilter{SellerID: 5, Limit: 500})
	require.NoError(t, err)
	require.Equal(t, domain.AdSortNewest, repo.adFilter.Sort)
	require.Equal(t, maxAdsLimit, repo.adFilter.Limit)
}

func TestSellerUsecase_Listings_InvalidSort(t *testing.T) {
	_, err := NewSellerUsecase(&sellerRepoStub{}).Listings(context.Background(), domain.SellerAdFilter{Sort: "rating_desc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Reviews(t *testing.T) {
	repo := &sellerRepoStub{}
	uc := NewSellerUsecase(repo)

	_, err := uc.Reviews(context.Background(), domain.ReviewFilter{SellerID: 5, Limit: 10, WithPhotos: true})
	require.NoError(t, err)
	require.Equal(t, domain.ReviewSortNewest, repo.reviewFilter.Sort)
	require.Equal(t, 10, repo.reviewFilter.Limit)
	require.True(t, repo.reviewFilter.WithPhotos)

	_, err = uc.Reviews(context.Background(), domain.ReviewFilter{Sort: "price_asc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Report(t *testing.T) {
	repo := &sellerRepoStub{}

	err := NewSellerUsecase(repo).Report(context.Background(), domain.SellerReport{
		SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonRude, Comment: "  Хамил в переписке ",
	})
	require.NoError(t, err)
	require.Equal(t, "Хамил в переписке", repo.report.Comment)
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
			repo := &sellerRepoStub{}
			require.ErrorIs(t, NewSellerUsecase(repo).Report(context.Background(), tt.report), tt.want)
			require.Nil(t, repo.report)
		})
	}
}

func TestSellerUsecase_Report_CommentLimitCountsRunes(t *testing.T) {
	err := NewSellerUsecase(&sellerRepoStub{}).Report(context.Background(), domain.SellerReport{
		SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther, Comment: strings.Repeat("я", maxReportCommentLength),
	})
	require.NoError(t, err)
}
