package usecase

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationRepository interface {
	ListPending(ctx context.Context, afterID int32, limit int) ([]domain.ModerationItem, error)
	LockRevision(ctx context.Context, id int32) (int64, error)
	Decide(ctx context.Context, id, moderatorID int32, revision int64, status, reason string) error
}

type ModerationUsecase struct {
	repo       ModerationRepository
	listings   ListingRepository
	tx         Transactor
	moderators []int32
}

func NewModerationUsecase(repo ModerationRepository, listings ListingRepository, tx Transactor, moderators []int32) *ModerationUsecase {
	return &ModerationUsecase{repo: repo, listings: listings, tx: tx, moderators: slices.Clone(moderators)}
}

func (uc *ModerationUsecase) allowed(userID int32) bool {
	return userID > 0 && slices.Contains(uc.moderators, userID)
}

func (uc *ModerationUsecase) List(ctx context.Context, userID, afterID int32, limit int) ([]domain.ModerationItem, error) {
	if !uc.allowed(userID) {
		return nil, domain.ErrForbidden
	}
	if afterID < 0 {
		afterID = 0
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return uc.repo.ListPending(ctx, afterID, limit)
}

func (uc *ModerationUsecase) Get(ctx context.Context, userID, id int32) (domain.ModerationListing, error) {
	if !uc.allowed(userID) {
		return domain.ModerationListing{}, domain.ErrForbidden
	}
	var result domain.ModerationListing
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		result.Revision, err = uc.repo.LockRevision(ctx, id)
		if err != nil {
			return err
		}
		result.Listing, err = uc.listings.Get(ctx, id, userID)
		if err != nil {
			return err
		}
		if result.Listing.Status != domain.AdStatusModeration {
			return domain.ErrModerationConflict
		}
		result.Listing.IsOwner = result.Listing.Seller.ID == userID
		return nil
	})
	return result, err
}

func (uc *ModerationUsecase) Decide(ctx context.Context, userID, id int32, revision int64, approve bool, reason string) error {
	if !uc.allowed(userID) {
		return domain.ErrForbidden
	}
	if revision <= 0 {
		return domain.ErrInvalidRevision
	}
	status := domain.AdStatusActive
	if approve {
		reason = ""
	} else {
		status = domain.AdStatusRejected
		reason = strings.TrimSpace(reason)
		if n := utf8.RuneCountInString(reason); n == 0 || n > 500 {
			return domain.ErrInvalidRejectionReason
		}
	}
	return uc.repo.Decide(ctx, id, userID, revision, status, reason)
}
