package usecase

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationRepository interface {
	CheckRole(ctx context.Context, actorID int32) error
	List(ctx context.Context, actorID int32, after *time.Time, afterID int32, limit int) ([]domain.ModerationRequest, error)
	LockPending(ctx context.Context, actorID, id int32) (int64, time.Time, error)
	Decide(ctx context.Context, actorID, id int32, revision int64, status, reason string) error
}

type ModerationUsecase struct {
	repo     ModerationRepository
	listings ListingRepository
	tx       Transactor
}

func NewModerationUsecase(repo ModerationRepository, listings ListingRepository, tx Transactor) *ModerationUsecase {
	return &ModerationUsecase{repo: repo, listings: listings, tx: tx}
}

func (uc *ModerationUsecase) List(ctx context.Context, actorID int32, cursor string, limit int) (domain.ModerationPage, error) {
	if limit <= 0 {
		limit = 20
	}
	var page domain.ModerationPage
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.CheckRole(ctx, actorID); err != nil {
			return err
		}
		var after *time.Time
		var afterID int32
		if cursor != "" {
			at, id, err := parseAdCursor(cursor)
			if err != nil {
				return err
			}
			after, afterID = &at, id
		}
		var err error
		page.Items, err = uc.repo.List(ctx, actorID, after, afterID, limit+1)
		if err != nil {
			return err
		}
		if len(page.Items) > limit {
			page.Items = page.Items[:limit]
			last := page.Items[limit-1]
			page.NextCursor = encodeAdCursor(last.RequestedAt, last.ID)
		}
		return nil
	})
	return page, err
}

func (uc *ModerationUsecase) Get(ctx context.Context, actorID, id int32) (domain.ModerationSnapshot, error) {
	var snapshot domain.ModerationSnapshot
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.CheckRole(ctx, actorID); err != nil {
			return err
		}
		var err error
		snapshot.Revision, snapshot.RequestedAt, err = uc.repo.LockPending(ctx, actorID, id)
		if err != nil {
			return err
		}
		snapshot.Listing, err = uc.listings.Get(ctx, id, actorID)
		return err
	})
	return snapshot, err
}

func (uc *ModerationUsecase) Decide(ctx context.Context, actorID, id int32, revision int64, approve bool, reason string) error {
	return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.repo.CheckRole(ctx, actorID); err != nil {
			return err
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
			if size := utf8.RuneCountInString(reason); size == 0 || size > domain.MaxRejectionReason || strings.ContainsRune(reason, 0) {
				return domain.ErrInvalidRejectionReason
			}
		}
		current, _, err := uc.repo.LockPending(ctx, actorID, id)
		if err != nil {
			return err
		}
		if current != revision {
			return domain.ErrModerationConflict
		}
		return uc.repo.Decide(ctx, actorID, id, revision, status, reason)
	})
}
