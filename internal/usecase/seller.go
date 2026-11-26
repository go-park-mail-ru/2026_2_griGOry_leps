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
	users   UserDirectory
}

func NewSellerUsecase(sellers SellerRepository, users UserDirectory) *SellerUsecase {
	return &SellerUsecase{sellers: sellers, users: users}
}

func (uc *SellerUsecase) Profile(ctx context.Context, id int32) (domain.SellerProfile, error) {
	card, err := requireSeller(ctx, uc.users, id)
	if err != nil {
		return domain.SellerProfile{}, err
	}
	p, err := uc.sellers.Get(ctx, id)
	if err != nil {
		return domain.SellerProfile{}, err
	}
	p.FirstName, p.Avatar, p.CreatedAt = card.FirstName, card.Avatar, card.CreatedAt
	p.Location, p.PhoneVerified = card.City, card.PhoneVerified
	return p, nil
}

func (uc *SellerUsecase) Stats(ctx context.Context, id int32) (domain.SellerStats, error) {
	return uc.sellers.Stats(ctx, id)
}

func (uc *SellerUsecase) Listings(ctx context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error) {
	if f.Sort == "" {
		f.Sort = domain.AdSortNewest
	}
	if !slices.Contains(adSorts, f.Sort) {
		return domain.SellerAdPage{}, domain.ErrInvalidSort
	}
	f.Limit = pageLimit(f.Limit)
	if _, err := requireSeller(ctx, uc.users, f.SellerID); err != nil {
		return domain.SellerAdPage{}, err
	}
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
	page, err := uc.sellers.ListReviews(ctx, f)
	if err != nil {
		return domain.ReviewPage{}, err
	}
	// Продавец и авторы отзывов запрашиваются одним вызовом; пропавший продавец — 404, пропавший автор — ошибка данных.
	ids := []int32{f.SellerID}
	for _, rv := range page.Items {
		ids = append(ids, rv.Author.ID)
	}
	cards, err := uc.users.Users(ctx, uniqueIDs(ids))
	if err != nil {
		return domain.ReviewPage{}, err
	}
	if _, ok := cards[f.SellerID]; !ok {
		return domain.ReviewPage{}, domain.ErrSellerNotFound
	}
	for i := range page.Items {
		card, ok := cards[page.Items[i].Author.ID]
		if !ok {
			return domain.ReviewPage{}, domain.ErrUserMissing
		}
		page.Items[i].Author.FirstName, page.Items[i].Author.Avatar = card.FirstName, card.Avatar
	}
	return page, nil
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
	if _, err := requireSeller(ctx, uc.users, report.SellerID); err != nil {
		return err
	}
	return uc.sellers.CreateReport(ctx, report)
}
