package usecase

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const maxReportCommentLength = 500

var (
	reviewSorts   = []string{domain.ReviewSortNewest, domain.ReviewSortRatingDesc, domain.ReviewSortRatingAsc}
	adSorts       = []string{domain.AdSortNewest, domain.AdSortPriceAsc, domain.AdSortPriceDesc}
	reportReasons = []string{
		domain.ReportReasonFraud, domain.ReportReasonProhibited, domain.ReportReasonRude, domain.ReportReasonOther,
	}
)

type SellerUsecase struct {
	sellers SellerRepository
}

func NewSellerUsecase(sellers SellerRepository) *SellerUsecase {
	return &SellerUsecase{sellers: sellers}
}

func (uc *SellerUsecase) Profile(ctx context.Context, id int32) (domain.SellerProfile, error) {
	return uc.sellers.Get(ctx, id)
}

func (uc *SellerUsecase) Listings(ctx context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error) {
	if f.Sort == "" {
		f.Sort = domain.AdSortNewest
	}
	if !slices.Contains(adSorts, f.Sort) {
		return domain.SellerAdPage{}, domain.ErrInvalidSort
	}
	f.Limit = pageLimit(f.Limit)
	return uc.sellers.ListAds(ctx, f)
}

func (uc *SellerUsecase) Reviews(ctx context.Context, f domain.ReviewFilter) (domain.ReviewPage, error) {
	if f.Sort == "" {
		f.Sort = domain.ReviewSortNewest
	}
	if !slices.Contains(reviewSorts, f.Sort) {
		return domain.ReviewPage{}, domain.ErrInvalidSort
	}
	f.Limit = pageLimit(f.Limit)
	return uc.sellers.ListReviews(ctx, f)
}

func (uc *SellerUsecase) Report(ctx context.Context, report domain.SellerReport) error {
	if !slices.Contains(reportReasons, report.Reason) {
		return domain.ErrInvalidReportReason
	}
	report.Comment = strings.TrimSpace(report.Comment)
	if utf8.RuneCountInString(report.Comment) > maxReportCommentLength {
		return domain.ErrReportCommentTooLong
	}
	if report.SellerID == report.ReporterID {
		return domain.ErrSelfReport
	}
	return uc.sellers.CreateReport(ctx, report)
}
