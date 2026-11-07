package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestSellerUsecase_Listings_Defaults(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().ListAds(gomock.Any(), domain.SellerAdFilter{SellerID: 5, Sort: domain.AdSortNewest, Limit: maxAdsLimit}).Return(domain.SellerAdPage{}, nil)

	_, err := NewSellerUsecase(repo).Listings(context.Background(), domain.SellerAdFilter{SellerID: 5, Limit: 500})
	require.NoError(t, err)

}

func TestSellerUsecase_Listings_InvalidSort(t *testing.T) {
	_, err := NewSellerUsecase(NewMockSellerRepository(gomock.NewController(t))).Listings(context.Background(), domain.SellerAdFilter{Sort: "rating_desc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Reviews(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().ListReviews(gomock.Any(), domain.ReviewFilter{SellerID: 5, Limit: 10, WithPhotos: true, Sort: domain.ReviewSortNewest}).Return(domain.ReviewPage{}, nil)
	uc := NewSellerUsecase(repo)

	_, err := uc.Reviews(context.Background(), domain.ReviewFilter{SellerID: 5, Limit: 10, WithPhotos: true})
	require.NoError(t, err)

	_, err = uc.Reviews(context.Background(), domain.ReviewFilter{Sort: "price_asc"})
	require.ErrorIs(t, err, domain.ErrInvalidSort)
}

func TestSellerUsecase_Report(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().CreateReport(gomock.Any(), domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonRude, Comment: "Хамил в переписке"}).Return(nil)

	err := NewSellerUsecase(repo).Report(context.Background(), domain.SellerReport{
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
			require.ErrorIs(t, NewSellerUsecase(repo).Report(context.Background(), tt.report), tt.want)
		})
	}
}

func TestSellerUsecase_Report_CommentLimitCountsRunes(t *testing.T) {
	repo := NewMockSellerRepository(gomock.NewController(t))
	repo.EXPECT().CreateReport(gomock.Any(), domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther, Comment: strings.Repeat("я", maxReportCommentLength)}).Return(nil)
	err := NewSellerUsecase(repo).Report(context.Background(), domain.SellerReport{
		SellerID: 5, ReporterID: 7, Reason: domain.ReportReasonOther, Comment: strings.Repeat("я", maxReportCommentLength),
	})
	require.NoError(t, err)
}
