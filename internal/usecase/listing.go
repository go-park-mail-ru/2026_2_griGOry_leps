package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	ListingPhotoURLPrefix = "/uploads/listings/"

	maxTitleLength       = 80
	maxDescriptionLength = 3000
	maxAddressLength     = 255
	stalePhotoAge        = 24 * time.Hour
	maxCategoryDepth     = 10
)

var (
	conditions   = map[string]bool{"new": true, "used_good": true, "used_repair": true}
	closeReasons = map[string]bool{"sold_on_goget": true, "sold_elsewhere": true, "changed_mind": true, "other": true}
)

type ListingUsecase struct {
	listings ListingRepository
	photos   FileStorage
	tx       Transactor
}

func NewListingUsecase(listings ListingRepository, photos FileStorage, tx Transactor) *ListingUsecase {
	return &ListingUsecase{listings: listings, photos: photos, tx: tx}
}

func (uc *ListingUsecase) Get(ctx context.Context, id, viewerID int32) (domain.Listing, error) {
	l, err := uc.listings.Get(ctx, id, viewerID)
	if err != nil {
		return domain.Listing{}, err
	}

	l.IsOwner = viewerID != 0 && viewerID == l.Seller.ID
	if l.IsOwner {
		return l, nil
	}
	if l.Status == domain.AdStatusDraft || l.Status == domain.AdStatusModeration || l.Status == domain.AdStatusRejected {
		return domain.Listing{}, domain.ErrListingNotFound
	}
	l.CloseReason = nil
	return l, nil
}

func (uc *ListingUsecase) Categories(ctx context.Context) ([]domain.CategoryNode, error) {
	categories, err := uc.listings.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	return categoryTree(categories), nil
}

func categoryTree(categories []domain.Category) []domain.CategoryNode {
	known := make(map[int32]bool, len(categories))
	for _, c := range categories {
		known[c.ID] = true
	}
	children := make(map[int32][]domain.Category)
	var roots []domain.Category
	for _, c := range categories {
		if c.ParentID != nil && known[*c.ParentID] && *c.ParentID != c.ID {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		} else {
			roots = append(roots, c)
		}
	}

	var build func(list []domain.Category, depth int) []domain.CategoryNode
	build = func(list []domain.Category, depth int) []domain.CategoryNode {
		nodes := make([]domain.CategoryNode, 0, len(list))
		for _, c := range list {
			node := domain.CategoryNode{ID: c.ID, Name: c.Name, Slug: c.Slug, Children: []domain.CategoryNode{}}
			if depth < maxCategoryDepth {
				node.Children = build(children[c.ID], depth+1)
			}
			nodes = append(nodes, node)
		}
		return nodes
	}
	return build(roots, 0)
}

func (uc *ListingUsecase) Create(ctx context.Context, userID int32, f domain.ListingFields, publish bool) (domain.Listing, error) {
	state := domain.ListingState{UserID: userID, Status: domain.AdStatusDraft}
	if err := uc.apply(ctx, &state, f); err != nil {
		return domain.Listing{}, err
	}
	if publish {
		if err := checkComplete(state); err != nil {
			return domain.Listing{}, err
		}
		state.Status = domain.AdStatusModeration
	}

	var id int32
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		if id, err = uc.listings.Create(ctx, state); err != nil {
			return err
		}
		if f.PhotoIDs != nil {
			_, err = uc.listings.SetPhotos(ctx, userID, id, state.PhotoIDs)
		}
		return err
	})
	if err != nil {
		return domain.Listing{}, err
	}
	return uc.Get(ctx, id, userID)
}

func (uc *ListingUsecase) Update(ctx context.Context, userID, id int32, f domain.ListingFields, publish *bool) (domain.Listing, error) {
	var removed []string
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		state, err := uc.ownState(ctx, userID, id)
		if err != nil {
			return err
		}
		if state.Status == domain.AdStatusSold {
			return domain.ErrListingSold
		}
		isDraft := state.Status == domain.AdStatusDraft
		if publish != nil && !isDraft {
			return domain.ErrListingNotDraft
		}

		if err := uc.apply(ctx, &state, f); err != nil {
			return err
		}
		toModeration := (publish != nil && *publish) || state.Status == domain.AdStatusRejected
		if !isDraft || toModeration {
			if err := checkComplete(state); err != nil {
				return err
			}
		}
		if toModeration {
			state.Status = domain.AdStatusModeration
		}

		if err := uc.listings.Update(ctx, id, state); err != nil {
			return err
		}
		if f.PhotoIDs != nil {
			removed, err = uc.listings.SetPhotos(ctx, userID, id, state.PhotoIDs)
		}
		return err
	})
	if err != nil {
		return domain.Listing{}, err
	}

	uc.removePhotos(ctx, removed)
	return uc.Get(ctx, id, userID)
}

func (uc *ListingUsecase) Delete(ctx context.Context, userID, id int32) error {
	var removed []string
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if _, err := uc.ownState(ctx, userID, id); err != nil {
			return err
		}
		var err error
		removed, err = uc.listings.Delete(ctx, id)
		return err
	})
	if err != nil {
		return err
	}

	uc.removePhotos(ctx, removed)
	return nil
}

func (uc *ListingUsecase) Close(ctx context.Context, userID, id int32, reason string) (domain.Listing, error) {
	if !closeReasons[reason] {
		return domain.Listing{}, domain.ErrInvalidCloseReason
	}

	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		state, err := uc.ownState(ctx, userID, id)
		if err != nil {
			return err
		}
		if state.Status != domain.AdStatusActive {
			return domain.ErrListingNotActive
		}
		return uc.listings.Close(ctx, id, reason)
	})
	if err != nil {
		return domain.Listing{}, err
	}
	return uc.Get(ctx, id, userID)
}

func (uc *ListingUsecase) UploadPhoto(ctx context.Context, userID int32, data []byte) (domain.ListingPhoto, error) {
	ext, err := imageExt(data)
	if err != nil {
		return domain.ListingPhoto{}, err
	}

	token, err := generateToken()
	if err != nil {
		return domain.ListingPhoto{}, err
	}
	name := token[:32] + ext

	if err := uc.photos.Save(ctx, name, data); err != nil {
		return domain.ListingPhoto{}, err
	}

	url := ListingPhotoURLPrefix + name
	id, err := uc.listings.CreatePhoto(ctx, userID, url)
	if err != nil {
		uc.removePhotos(ctx, []string{url})
		return domain.ListingPhoto{}, err
	}
	return domain.ListingPhoto{ID: id, URL: url}, nil
}

func (uc *ListingUsecase) RunMaintenance(ctx context.Context, now time.Time) error {
	approved, err := uc.listings.ApproveModeration(ctx)
	if err != nil {
		return err
	}
	if approved > 0 {
		slog.InfoContext(ctx, "listings approved", "count", approved)
	}

	stale, err := uc.listings.DeleteStalePhotos(ctx, now.Add(-stalePhotoAge))
	if err != nil {
		return err
	}
	uc.removePhotos(ctx, stale)
	return nil
}

func (uc *ListingUsecase) ownState(ctx context.Context, userID, id int32) (domain.ListingState, error) {
	state, err := uc.listings.GetStateForUpdate(ctx, id)
	if err != nil {
		return domain.ListingState{}, err
	}
	if state.UserID != userID {
		return domain.ListingState{}, domain.ErrForbidden
	}
	return state, nil
}

func (uc *ListingUsecase) apply(ctx context.Context, state *domain.ListingState, f domain.ListingFields) error {
	if f.CategoryID.Set {
		if f.CategoryID.Null {
			state.CategoryID = nil
		} else {
			ok, err := uc.listings.IsLeafCategory(ctx, f.CategoryID.Value)
			if err != nil {
				return err
			}
			if !ok {
				return domain.ErrInvalidCategory
			}
			id := f.CategoryID.Value
			state.CategoryID = &id
		}
	}

	if f.Title != nil {
		title := strings.TrimSpace(*f.Title)
		if n := utf8.RuneCountInString(title); n == 0 || n > maxTitleLength {
			return domain.ErrInvalidTitle
		}
		state.Title = title
	}

	if f.PhotoIDs != nil {
		if err := checkPhotoIDs(*f.PhotoIDs); err != nil {
			return err
		}
		state.PhotoIDs = *f.PhotoIDs
	}

	if f.Description != nil {
		description := strings.TrimSpace(*f.Description)
		if utf8.RuneCountInString(description) > maxDescriptionLength {
			return domain.ErrDescriptionTooLong
		}
		state.Description = description
	}

	if f.Price.Set {
		if f.Price.Null {
			state.Price = nil
		} else {
			price, err := parsePrice(f.Price.Value)
			if err != nil {
				return err
			}
			state.Price = &price
		}
	}

	if f.Condition.Set {
		if f.Condition.Null {
			state.Condition = nil
		} else {
			if !conditions[f.Condition.Value] {
				return domain.ErrInvalidCondition
			}
			condition := f.Condition.Value
			state.Condition = &condition
		}
	}

	if f.Address.Set {
		if f.Address.Null {
			state.Address = nil
		} else {
			address := strings.TrimSpace(f.Address.Value)
			if address == "" {
				return domain.ErrAddressRequired
			}
			if utf8.RuneCountInString(address) > maxAddressLength {
				return domain.ErrAddressTooLong
			}
			state.Address = &address
		}
	}

	if f.IsFree != nil {
		state.IsFree = *f.IsFree
	}
	if state.IsFree {
		var zero int64
		state.Price = &zero
	}
	if f.IsNegotiable != nil {
		state.IsNegotiable = *f.IsNegotiable
	}
	if f.HasDelivery != nil {
		state.HasDelivery = *f.HasDelivery
	}
	return nil
}

func checkPhotoIDs(ids []int32) error {
	if len(ids) > domain.MaxListingPhotos {
		return domain.ErrTooManyPhotos
	}
	seen := make(map[int32]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			return domain.ErrInvalidPhotoID
		}
		seen[id] = true
	}
	return nil
}

func checkComplete(s domain.ListingState) error {
	switch {
	case s.CategoryID == nil:
		return domain.ErrInvalidCategory
	case s.Title == "":
		return domain.ErrInvalidTitle
	case len(s.PhotoIDs) == 0:
		return domain.ErrPhotosRequired
	case s.Price == nil:
		return domain.ErrInvalidPrice
	case s.Condition == nil:
		return domain.ErrInvalidCondition
	case s.Address == nil:
		return domain.ErrAddressRequired
	}
	return nil
}

func (uc *ListingUsecase) removePhotos(ctx context.Context, urls []string) {
	for _, url := range urls {
		name, ok := strings.CutPrefix(url, ListingPhotoURLPrefix)
		if !ok {
			continue
		}
		if err := uc.photos.Delete(ctx, name); err != nil {
			slog.WarnContext(ctx, "delete listing photo", "file", name, "err", err)
		}
	}
}
