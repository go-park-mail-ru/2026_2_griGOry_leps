package usecase

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type photoFake struct {
	userID    int32
	listingID int32
	url       string
	createdAt time.Time
}

type listingRepoFake struct {
	mu     sync.Mutex
	lastID int32
	states map[int32]domain.ListingState
	photos map[int32]*photoFake
	leaves map[int32]bool
	closed map[int32]string

	categories []domain.Category
}

func newListingRepoFake() *listingRepoFake {
	return &listingRepoFake{
		states: make(map[int32]domain.ListingState),
		photos: make(map[int32]*photoFake),
		leaves: map[int32]bool{12: true},
		closed: make(map[int32]string),
	}
}

func (r *listingRepoFake) Get(_ context.Context, id, _ int32) (domain.Listing, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.states[id]
	if !ok {
		return domain.Listing{}, domain.ErrListingNotFound
	}
	l := domain.Listing{
		ID: id, Title: s.Title, Description: s.Description, Price: s.Price, IsFree: s.IsFree,
		Condition: s.Condition, Address: s.Address, Status: s.Status, Seller: domain.Seller{ID: s.UserID},
	}
	if reason, ok := r.closed[id]; ok {
		l.CloseReason = &reason
	}
	for _, photoID := range s.PhotoIDs {
		l.Photos = append(l.Photos, domain.ListingPhoto{ID: photoID, URL: r.photos[photoID].url})
	}
	return l, nil
}

func (r *listingRepoFake) GetStateForUpdate(_ context.Context, id int32) (domain.ListingState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.states[id]
	if !ok {
		return domain.ListingState{}, domain.ErrListingNotFound
	}
	return s, nil
}

func (r *listingRepoFake) Create(_ context.Context, s domain.ListingState) (int32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastID++
	s.PhotoIDs = nil
	r.states[r.lastID] = s
	return r.lastID, nil
}

func (r *listingRepoFake) Update(_ context.Context, id int32, s domain.ListingState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s.PhotoIDs = r.states[id].PhotoIDs
	r.states[id] = s
	return nil
}

func (r *listingRepoFake) SetPhotos(_ context.Context, userID, listingID int32, ids []int32) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	keep := make(map[int32]bool, len(ids))
	for _, id := range ids {
		p, ok := r.photos[id]
		if !ok || p.userID != userID || (p.listingID != 0 && p.listingID != listingID) {
			return nil, domain.ErrInvalidPhotoID
		}
		keep[id] = true
	}

	var removed []string
	for id, p := range r.photos {
		if p.listingID == listingID && !keep[id] {
			removed = append(removed, p.url)
			delete(r.photos, id)
		}
	}
	for _, id := range ids {
		r.photos[id].listingID = listingID
	}
	s := r.states[listingID]
	s.PhotoIDs = append([]int32(nil), ids...)
	r.states[listingID] = s
	return removed, nil
}

func (r *listingRepoFake) Delete(_ context.Context, id int32) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var urls []string
	for photoID, p := range r.photos {
		if p.listingID == id {
			urls = append(urls, p.url)
			delete(r.photos, photoID)
		}
	}
	delete(r.states, id)
	return urls, nil
}

func (r *listingRepoFake) Close(_ context.Context, id int32, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s := r.states[id]
	s.Status = domain.AdStatusClosed
	r.states[id] = s
	r.closed[id] = reason
	return nil
}

func (r *listingRepoFake) IsLeafCategory(_ context.Context, id int32) (bool, error) {
	return r.leaves[id], nil
}

func (r *listingRepoFake) ListCategories(context.Context) ([]domain.Category, error) {
	return r.categories, nil
}

func (r *listingRepoFake) CreatePhoto(_ context.Context, userID int32, url string) (int32, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id := int32(500 + len(r.photos) + 1)
	r.photos[id] = &photoFake{userID: userID, url: url, createdAt: time.Now()}
	return id, nil
}

func (r *listingRepoFake) ApproveModeration(_ context.Context) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var n int64
	for id, s := range r.states {
		if s.Status == domain.AdStatusModeration {
			s.Status = domain.AdStatusActive
			r.states[id] = s
			n++
		}
	}
	return n, nil
}

func (r *listingRepoFake) DeleteStalePhotos(_ context.Context, before time.Time) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var urls []string
	for id, p := range r.photos {
		if p.listingID == 0 && p.createdAt.Before(before) {
			urls = append(urls, p.url)
			delete(r.photos, id)
		}
	}
	return urls, nil
}

const (
	authorID int32 = 1
	otherID  int32 = 2
)

type listingEnv struct {
	uc      *ListingUsecase
	repo    *listingRepoFake
	storage *storageFake
}

func newListingEnv() listingEnv {
	repo := newListingRepoFake()
	storage := newStorageFake()
	return listingEnv{uc: NewListingUsecase(repo, storage, txFake{}), repo: repo, storage: storage}
}

func (e listingEnv) photo(t *testing.T, userID int32) int32 {
	t.Helper()
	p, err := e.uc.UploadPhoto(context.Background(), userID, pngBytes(t))
	require.NoError(t, err)
	return p.ID
}

func set[T any](v T) domain.Optional[T] { return domain.Optional[T]{Set: true, Value: v} }

func null[T any]() domain.Optional[T] { return domain.Optional[T]{Set: true, Null: true} }

func (e listingEnv) fullFields(t *testing.T) domain.ListingFields {
	return domain.ListingFields{
		CategoryID: set(int32(12)),
		Title:      ptr("  Велосипед  "),
		PhotoIDs:   &[]int32{e.photo(t, authorID), e.photo(t, authorID)},
		Price:      set("15000.5"),
		Condition:  set("used_good"),
		Address:    set("Москва, м. Сокол"),
	}
}

func (e listingEnv) active(t *testing.T) domain.Listing {
	t.Helper()
	l, err := e.uc.Create(context.Background(), authorID, e.fullFields(t), true)
	require.NoError(t, err)
	require.NoError(t, e.uc.RunMaintenance(context.Background(), time.Now()))
	return l
}

func TestListing_CreatePublishGoesToModeration(t *testing.T) {
	e := newListingEnv()

	l, err := e.uc.Create(context.Background(), authorID, e.fullFields(t), true)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, l.Status)
	require.Equal(t, "Велосипед", l.Title)
	require.Equal(t, int64(1500050), *l.Price)
	require.Len(t, l.Photos, 2)
	require.True(t, l.IsOwner)
}

func TestListing_CreatePublishChecksAllRules(t *testing.T) {
	cases := []struct {
		name   string
		change func(f *domain.ListingFields)
		err    error
	}{
		{"нет категории", func(f *domain.ListingFields) { f.CategoryID = domain.Optional[int32]{} }, domain.ErrInvalidCategory},
		{"не лист", func(f *domain.ListingFields) { f.CategoryID = set(int32(1)) }, domain.ErrInvalidCategory},
		{"пустой заголовок", func(f *domain.ListingFields) { f.Title = ptr("   ") }, domain.ErrInvalidTitle},
		{"длинный заголовок", func(f *domain.ListingFields) { f.Title = ptr(strings.Repeat("я", 81)) }, domain.ErrInvalidTitle},
		{"без фото", func(f *domain.ListingFields) { f.PhotoIDs = &[]int32{} }, domain.ErrPhotosRequired},
		{"фото дважды", func(f *domain.ListingFields) { f.PhotoIDs = &[]int32{(*f.PhotoIDs)[0], (*f.PhotoIDs)[0]} }, domain.ErrInvalidPhotoID},
		{"чужое фото", func(f *domain.ListingFields) { *f.PhotoIDs = append(*f.PhotoIDs, 999) }, domain.ErrInvalidPhotoID},
		{"длинное описание", func(f *domain.ListingFields) { f.Description = ptr(strings.Repeat("а", 3001)) }, domain.ErrDescriptionTooLong},
		{"без цены", func(f *domain.ListingFields) { f.Price = domain.Optional[string]{} }, domain.ErrInvalidPrice},
		{"цена буквами", func(f *domain.ListingFields) { f.Price = set("сто") }, domain.ErrInvalidPrice},
		{"состояние", func(f *domain.ListingFields) { f.Condition = set("broken") }, domain.ErrInvalidCondition},
		{"без адреса", func(f *domain.ListingFields) { f.Address = set("  ") }, domain.ErrAddressRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newListingEnv()
			f := e.fullFields(t)
			tc.change(&f)

			_, err := e.uc.Create(context.Background(), authorID, f, true)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestListing_FreeListingNeedsNoPrice(t *testing.T) {
	e := newListingEnv()
	f := e.fullFields(t)
	f.Price = domain.Optional[string]{}
	f.IsFree = ptr(true)

	l, err := e.uc.Create(context.Background(), authorID, f, true)
	require.NoError(t, err)
	require.Equal(t, int64(0), *l.Price)
}

func TestListing_DraftKeepsOnlyPassedFields(t *testing.T) {
	e := newListingEnv()

	l, err := e.uc.Create(context.Background(), authorID, domain.ListingFields{Title: ptr("Диван")}, false)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusDraft, l.Status)
	require.Nil(t, l.Price)
	require.Empty(t, l.Photos)
}

func TestListing_GetHidesDraftsAndCloseReasonFromOthers(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()

	draft, err := e.uc.Create(ctx, authorID, domain.ListingFields{Title: ptr("Диван")}, false)
	require.NoError(t, err)
	_, err = e.uc.Get(ctx, draft.ID, otherID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
	_, err = e.uc.Get(ctx, draft.ID, 0)
	require.ErrorIs(t, err, domain.ErrListingNotFound)

	l := e.active(t)
	_, err = e.uc.Close(ctx, authorID, l.ID, "sold_elsewhere")
	require.NoError(t, err)

	forOther, err := e.uc.Get(ctx, l.ID, otherID)
	require.NoError(t, err)
	require.False(t, forOther.IsOwner)
	require.Nil(t, forOther.CloseReason)

	forAuthor, err := e.uc.Get(ctx, l.ID, authorID)
	require.NoError(t, err)
	require.True(t, forAuthor.IsOwner)
	require.Equal(t, "sold_elsewhere", *forAuthor.CloseReason)
}

func TestListing_OnlyAuthorCanChange(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l := e.active(t)

	_, err := e.uc.Update(ctx, otherID, l.ID, domain.ListingFields{Title: ptr("Моё")}, nil)
	require.ErrorIs(t, err, domain.ErrForbidden)
	_, err = e.uc.Close(ctx, otherID, l.ID, "other")
	require.ErrorIs(t, err, domain.ErrForbidden)
	require.ErrorIs(t, e.uc.Delete(ctx, otherID, l.ID), domain.ErrForbidden)

	_, err = e.uc.Update(ctx, authorID, 404, domain.ListingFields{}, nil)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestListing_UpdateActiveCannotClearRequired(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l := e.active(t)

	_, err := e.uc.Update(ctx, authorID, l.ID, domain.ListingFields{Address: null[string]()}, nil)
	require.ErrorIs(t, err, domain.ErrAddressRequired)
	_, err = e.uc.Update(ctx, authorID, l.ID, domain.ListingFields{PhotoIDs: &[]int32{}}, nil)
	require.ErrorIs(t, err, domain.ErrPhotosRequired)
	_, err = e.uc.Update(ctx, authorID, l.ID, domain.ListingFields{}, ptr(true))
	require.ErrorIs(t, err, domain.ErrListingNotDraft)

	updated, err := e.uc.Update(ctx, authorID, l.ID, domain.ListingFields{Description: ptr("")}, nil)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusActive, updated.Status)
	require.Empty(t, updated.Description)
}

func TestListing_UpdateDraftClearsAndPublishes(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()

	draft, err := e.uc.Create(ctx, authorID, e.fullFields(t), false)
	require.NoError(t, err)

	cleared, err := e.uc.Update(ctx, authorID, draft.ID, domain.ListingFields{Address: null[string](), PhotoIDs: &[]int32{}}, nil)
	require.NoError(t, err)
	require.Nil(t, cleared.Address)
	require.Empty(t, cleared.Photos)
	require.Len(t, e.storage.files, 0)

	_, err = e.uc.Update(ctx, authorID, draft.ID, domain.ListingFields{Title: ptr("Новое")}, ptr(true))
	require.ErrorIs(t, err, domain.ErrPhotosRequired)
	unchanged, err := e.uc.Get(ctx, draft.ID, authorID)
	require.NoError(t, err)
	require.Equal(t, "Велосипед", unchanged.Title)

	published, err := e.uc.Update(ctx, authorID, draft.ID, domain.ListingFields{
		PhotoIDs: &[]int32{e.photo(t, authorID)},
		Address:  set("Химки"),
	}, ptr(true))
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, published.Status)
}

func TestListing_UpdateSoldIsConflict(t *testing.T) {
	e := newListingEnv()
	l := e.active(t)
	s := e.repo.states[l.ID]
	s.Status = domain.AdStatusSold
	e.repo.states[l.ID] = s

	_, err := e.uc.Update(context.Background(), authorID, l.ID, domain.ListingFields{Title: ptr("Ещё")}, nil)
	require.ErrorIs(t, err, domain.ErrListingSold)
}

func TestListing_ReplacingPhotosDeletesFiles(t *testing.T) {
	e := newListingEnv()
	l := e.active(t)
	keep := l.Photos[1].ID

	updated, err := e.uc.Update(context.Background(), authorID, l.ID, domain.ListingFields{PhotoIDs: &[]int32{keep}}, nil)
	require.NoError(t, err)
	require.Len(t, updated.Photos, 1)
	require.Len(t, e.storage.files, 1)
}

func TestListing_CloseOnlyActive(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()

	_, err := e.uc.Close(ctx, authorID, 1, "lost")
	require.ErrorIs(t, err, domain.ErrInvalidCloseReason)

	draft, err := e.uc.Create(ctx, authorID, domain.ListingFields{Title: ptr("Диван")}, false)
	require.NoError(t, err)
	_, err = e.uc.Close(ctx, authorID, draft.ID, "other")
	require.ErrorIs(t, err, domain.ErrListingNotActive)

	l := e.active(t)
	closed, err := e.uc.Close(ctx, authorID, l.ID, "changed_mind")
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusClosed, closed.Status)

	_, err = e.uc.Close(ctx, authorID, l.ID, "other")
	require.ErrorIs(t, err, domain.ErrListingNotActive)
}

func TestListing_DeleteRemovesPhotos(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l := e.active(t)

	require.NoError(t, e.uc.Delete(ctx, authorID, l.ID))
	require.Empty(t, e.storage.files)
	_, err := e.uc.Get(ctx, l.ID, authorID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestListing_UploadPhotoRejectsNonImage(t *testing.T) {
	e := newListingEnv()

	_, err := e.uc.UploadPhoto(context.Background(), authorID, []byte("not an image"))
	require.ErrorIs(t, err, domain.ErrUnsupportedImage)
	require.Empty(t, e.storage.files)
}

func TestListing_MaintenanceDeletesStalePhotos(t *testing.T) {
	e := newListingEnv()
	e.photo(t, authorID)

	require.NoError(t, e.uc.RunMaintenance(context.Background(), time.Now()))
	require.Len(t, e.storage.files, 1)

	require.NoError(t, e.uc.RunMaintenance(context.Background(), time.Now().Add(25*time.Hour)))
	require.Empty(t, e.storage.files)
}

func TestListing_CategoriesBuildsTree(t *testing.T) {
	e := newListingEnv()
	parent := int32(1)
	missing := int32(99)
	e.repo.categories = []domain.Category{
		{ID: 1, Name: "Спорт", Slug: "sport"},
		{ID: 2, ParentID: &parent, Name: "Велосипеды", Slug: "bikes"},
		{ID: 3, ParentID: &missing, Name: "Сироты", Slug: "orphans"},
	}

	tree, err := e.uc.Categories(context.Background())
	require.NoError(t, err)
	require.Len(t, tree, 2)
	require.Equal(t, "sport", tree[0].Slug)
	require.Len(t, tree[0].Children, 1)
	require.Equal(t, int32(2), tree[0].Children[0].ID)
	require.NotNil(t, tree[0].Children[0].Children)
	require.Equal(t, "orphans", tree[1].Slug)
}

func TestListing_GetHidesRejectedFromOthers(t *testing.T) {
	e := newListingEnv()
	ctx := context.Background()
	l := e.active(t)
	s := e.repo.states[l.ID]
	s.Status = domain.AdStatusRejected
	e.repo.states[l.ID] = s

	_, err := e.uc.Get(ctx, l.ID, otherID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)

	own, err := e.uc.Get(ctx, l.ID, authorID)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusRejected, own.Status)
}

func TestListing_UpdateRejectedGoesBackToModeration(t *testing.T) {
	e := newListingEnv()
	l := e.active(t)
	s := e.repo.states[l.ID]
	s.Status = domain.AdStatusRejected
	e.repo.states[l.ID] = s

	_, err := e.uc.Update(context.Background(), authorID, l.ID, domain.ListingFields{PhotoIDs: &[]int32{}}, nil)
	require.ErrorIs(t, err, domain.ErrPhotosRequired)

	updated, err := e.uc.Update(context.Background(), authorID, l.ID, domain.ListingFields{Title: ptr("Исправил")}, nil)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, updated.Status)
	require.Equal(t, "Исправил", updated.Title)
}
