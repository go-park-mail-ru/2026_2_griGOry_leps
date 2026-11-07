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

const (
	authorID int32 = 1
	otherID  int32 = 2
)

type listingTestEnv struct {
	uc      *ListingUsecase
	repo    *MockListingRepository
	storage *MockFileStorage
	tx      *MockTransactor
}

func newListingTestEnv(t *testing.T) *listingTestEnv {
	t.Helper()
	c := gomock.NewController(t)
	e := &listingTestEnv{repo: NewMockListingRepository(c), storage: NewMockFileStorage(c), tx: NewMockTransactor(c)}
	e.uc = NewListingUsecase(e.repo, e.storage, e.tx)
	return e
}

func set[T any](v T) domain.Optional[T] { return domain.Optional[T]{Set: true, Value: v} }
func null[T any]() domain.Optional[T]   { return domain.Optional[T]{Set: true, Null: true} }
func listingFields() domain.ListingFields {
	return domain.ListingFields{CategoryID: set(int32(12)), Title: ptr("  Bike  "), PhotoIDs: &[]int32{1, 2}, Price: set("15000.5"), Condition: set("used_good"), Address: set("Moscow")}
}

func listingState(status string) domain.ListingState {
	return domain.ListingState{UserID: authorID, Status: status, CategoryID: ptr(int32(12)), Title: "Bike", PhotoIDs: []int32{1, 2}, Price: ptr(int64(1500050)), Condition: ptr("used_good"), Address: ptr("Moscow")}
}

func expectListingState(e *listingTestEnv, state domain.ListingState) {
	expectTransaction(e.tx)
	e.repo.EXPECT().GetStateForUpdate(gomock.Any(), int32(7)).Return(state, nil)
}

func expectListingGet(e *listingTestEnv, l domain.Listing) {
	e.repo.EXPECT().Get(gomock.Any(), int32(7), authorID).Return(l, nil)
}

func expectListingCreate(e *listingTestEnv, state domain.ListingState, photos bool, result domain.Listing) {
	expectTransaction(e.tx)
	create := e.repo.EXPECT().Create(gomock.Any(), state).Return(int32(7), nil)
	if photos {
		e.repo.EXPECT().SetPhotos(gomock.Any(), authorID, int32(7), state.PhotoIDs).Return(nil, nil).After(create)
	}
	expectListingGet(e, result)
}

func TestListing_CreatePublishGoesToModeration(t *testing.T) {
	e := newListingTestEnv(t)
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(12)).Return(true, nil)
	s := listingState(domain.AdStatusModeration)
	want := domain.Listing{ID: 7, Title: s.Title, Price: s.Price, Status: s.Status, Seller: domain.Seller{ID: authorID}, Photos: []domain.ListingPhoto{{ID: 1}, {ID: 2}}}
	expectListingCreate(e, s, true, want)
	l, err := e.uc.Create(context.Background(), authorID, listingFields(), true)
	require.NoError(t, err)
	want.IsOwner = true
	require.Equal(t, want, l)
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
			e := newListingTestEnv(t)
			f := listingFields()
			tc.change(&f)
			if f.CategoryID.Set {
				e.repo.EXPECT().IsLeafCategory(gomock.Any(), f.CategoryID.Value).Return(f.CategoryID.Value == 12, nil)
			}
			if tc.name == "чужое фото" {
				expectTransaction(e.tx)
				state := listingState(domain.AdStatusModeration)
				state.PhotoIDs = *f.PhotoIDs
				e.repo.EXPECT().Create(gomock.Any(), state).Return(int32(7), nil)
				e.repo.EXPECT().SetPhotos(gomock.Any(), authorID, int32(7), *f.PhotoIDs).Return(nil, domain.ErrInvalidPhotoID)
			}

			_, err := e.uc.Create(context.Background(), authorID, f, true)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestListing_FreeListingNeedsNoPrice(t *testing.T) {
	e := newListingTestEnv(t)
	f := listingFields()
	f.Price = domain.Optional[string]{}
	f.IsFree = ptr(true)
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(12)).Return(true, nil)
	state := listingState(domain.AdStatusModeration)
	state.IsFree = true
	state.Price = ptr(int64(0))
	expectListingCreate(e, state, true, domain.Listing{Price: state.Price, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Create(context.Background(), authorID, f, true)
	require.NoError(t, err)
	require.Equal(t, int64(0), *l.Price)
}

func TestListing_DraftKeepsOnlyPassedFields(t *testing.T) {
	e := newListingTestEnv(t)
	state := domain.ListingState{UserID: authorID, Status: domain.AdStatusDraft, Title: "Sofa"}
	expectListingCreate(e, state, false, domain.Listing{Status: state.Status, Title: state.Title, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Create(context.Background(), authorID, domain.ListingFields{Title: ptr("Sofa")}, false)
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusDraft, l.Status)
	require.Nil(t, l.Price)
	require.Empty(t, l.Photos)
}

func TestListing_GetHidesDraftsAndCloseReasonFromOthers(t *testing.T) {
	e := newListingTestEnv(t)
	for _, viewer := range []int32{otherID, 0} {
		e.repo.EXPECT().Get(gomock.Any(), int32(7), viewer).Return(domain.Listing{Status: domain.AdStatusDraft, Seller: domain.Seller{ID: authorID}}, nil)
		_, err := e.uc.Get(context.Background(), 7, viewer)
		require.ErrorIs(t, err, domain.ErrListingNotFound)
	}
	for _, viewer := range []int32{otherID, authorID} {
		e.repo.EXPECT().Get(gomock.Any(), int32(7), viewer).Return(domain.Listing{Status: domain.AdStatusClosed, Seller: domain.Seller{ID: authorID}, CloseReason: ptr("sold_elsewhere")}, nil)
		l, err := e.uc.Get(context.Background(), 7, viewer)
		require.NoError(t, err)
		require.Equal(t, viewer == authorID, l.IsOwner)
		if viewer == authorID {
			require.Equal(t, "sold_elsewhere", *l.CloseReason)
		} else {
			require.Nil(t, l.CloseReason)
		}
	}
}

func TestListing_OnlyAuthorCanChange(t *testing.T) {
	e := newListingTestEnv(t)
	state := listingState(domain.AdStatusActive)
	expectListingState(e, state)
	_, err := e.uc.Update(context.Background(), otherID, 7, domain.ListingFields{Title: ptr("Mine")}, nil)
	require.ErrorIs(t, err, domain.ErrForbidden)
	expectListingState(e, state)
	_, err = e.uc.Close(context.Background(), otherID, 7, "other")
	require.ErrorIs(t, err, domain.ErrForbidden)
	expectListingState(e, state)
	require.ErrorIs(t, e.uc.Delete(context.Background(), otherID, 7), domain.ErrForbidden)
	expectTransaction(e.tx)
	e.repo.EXPECT().GetStateForUpdate(gomock.Any(), int32(404)).Return(domain.ListingState{}, domain.ErrListingNotFound)
	_, err = e.uc.Update(context.Background(), authorID, 404, domain.ListingFields{}, nil)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestListing_UpdateActiveCannotClearRequired(t *testing.T) {
	e := newListingTestEnv(t)
	s := listingState(domain.AdStatusActive)
	expectListingState(e, s)
	_, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Address: null[string]()}, nil)
	require.ErrorIs(t, err, domain.ErrAddressRequired)
	expectListingState(e, s)
	_, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{PhotoIDs: &[]int32{}}, nil)
	require.ErrorIs(t, err, domain.ErrPhotosRequired)
	expectListingState(e, s)
	_, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{}, ptr(true))
	require.ErrorIs(t, err, domain.ErrListingNotDraft)
	expectListingState(e, s)
	e.repo.EXPECT().Update(gomock.Any(), int32(7), s).Return(nil)
	expectListingGet(e, domain.Listing{Status: s.Status, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Description: ptr("")}, nil)
	require.NoError(t, err)
	require.Equal(t, s.Status, l.Status)
	require.Empty(t, l.Description)
}

func TestListing_UpdateDraftClearsAndPublishes(t *testing.T) {
	e := newListingTestEnv(t)
	s := listingState(domain.AdStatusDraft)
	expectListingState(e, s)
	cleared := s
	cleared.Address = nil
	cleared.PhotoIDs = []int32{}
	e.repo.EXPECT().Update(gomock.Any(), int32(7), cleared).Return(nil)
	e.repo.EXPECT().SetPhotos(gomock.Any(), authorID, int32(7), []int32{}).Return([]string{ListingPhotoURLPrefix + "a.png"}, nil)
	e.storage.EXPECT().Delete(gomock.Any(), "a.png").Return(nil)
	expectListingGet(e, domain.Listing{Status: s.Status, Title: s.Title, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Address: null[string](), PhotoIDs: &[]int32{}}, nil)
	require.NoError(t, err)
	require.Nil(t, l.Address)
	require.Empty(t, l.Photos)
	expectListingState(e, cleared)
	_, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Title: ptr("New")}, ptr(true))
	require.ErrorIs(t, err, domain.ErrPhotosRequired)
	expectListingState(e, cleared)
	published := cleared
	published.Address = ptr("Khimki")
	published.PhotoIDs = []int32{3}
	published.Status = domain.AdStatusModeration
	e.repo.EXPECT().Update(gomock.Any(), int32(7), published).Return(nil)
	e.repo.EXPECT().SetPhotos(gomock.Any(), authorID, int32(7), []int32{3}).Return(nil, nil)
	expectListingGet(e, domain.Listing{Status: published.Status, Seller: domain.Seller{ID: authorID}})
	l, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Address: set("Khimki"), PhotoIDs: &[]int32{3}}, ptr(true))
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusModeration, l.Status)
}

func TestListing_UpdateSoldIsConflict(t *testing.T) {
	e := newListingTestEnv(t)
	expectListingState(e, listingState(domain.AdStatusSold))
	_, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Title: ptr("More")}, nil)
	require.ErrorIs(t, err, domain.ErrListingSold)
}

func TestListing_ReplacingPhotosDeletesFiles(t *testing.T) {
	e := newListingTestEnv(t)
	s := listingState(domain.AdStatusActive)
	expectListingState(e, s)
	s.PhotoIDs = []int32{2}
	e.repo.EXPECT().Update(gomock.Any(), int32(7), s).Return(nil)
	e.repo.EXPECT().SetPhotos(gomock.Any(), authorID, int32(7), []int32{2}).Return([]string{ListingPhotoURLPrefix + "old.png"}, nil)
	e.storage.EXPECT().Delete(gomock.Any(), "old.png").Return(nil)
	expectListingGet(e, domain.Listing{Photos: []domain.ListingPhoto{{ID: 2}}, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{PhotoIDs: &[]int32{2}}, nil)
	require.NoError(t, err)
	require.Len(t, l.Photos, 1)
}

func expectListingClose(e *listingTestEnv, status string) {
	expectListingState(e, listingState(status))
	e.repo.EXPECT().ActiveOrder(gomock.Any(), int32(7)).Return(int32(0), false, nil)
	e.repo.EXPECT().Close(gomock.Any(), int32(7), "changed_mind").Return(nil)
	expectListingGet(e, domain.Listing{Status: domain.AdStatusClosed, Seller: domain.Seller{ID: authorID}})
}

func TestListing_CloseOnlyActiveOrModeration(t *testing.T) {
	e := newListingTestEnv(t)
	_, err := e.uc.Close(context.Background(), authorID, 7, "lost")
	require.ErrorIs(t, err, domain.ErrInvalidCloseReason)
	expectListingState(e, listingState(domain.AdStatusDraft))
	_, err = e.uc.Close(context.Background(), authorID, 7, "other")
	require.ErrorIs(t, err, domain.ErrListingNotActive)
	expectListingClose(e, domain.AdStatusActive)
	l, err := e.uc.Close(context.Background(), authorID, 7, "changed_mind")
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusClosed, l.Status)
	expectListingState(e, listingState(domain.AdStatusClosed))
	_, err = e.uc.Close(context.Background(), authorID, 7, "other")
	require.ErrorIs(t, err, domain.ErrListingNotActive)
}

func TestListing_UnpaidOrderBlocksCloseAndDelete(t *testing.T) {
	e := newListingTestEnv(t)
	expectListingState(e, listingState(domain.AdStatusActive))
	e.repo.EXPECT().ActiveOrder(gomock.Any(), int32(7)).Return(int32(7), true, nil)
	_, err := e.uc.Close(context.Background(), authorID, 7, "other")
	var orderErr *domain.ListingOrderError
	require.ErrorAs(t, err, &orderErr)
	require.Equal(t, int32(10007), orderErr.OrderNumber)
	expectListingState(e, listingState(domain.AdStatusActive))
	e.repo.EXPECT().ActiveOrder(gomock.Any(), int32(7)).Return(int32(7), true, nil)
	require.ErrorAs(t, e.uc.Delete(context.Background(), authorID, 7), &orderErr)
	expectListingState(e, listingState(domain.AdStatusActive))
	e.repo.EXPECT().ActiveOrder(gomock.Any(), int32(7)).Return(int32(0), false, nil)
	e.repo.EXPECT().Delete(gomock.Any(), int32(7)).Return(nil, nil)
	require.NoError(t, e.uc.Delete(context.Background(), authorID, 7))
}

func TestListing_DeliveryOnlyWhereCategoryAllows(t *testing.T) {
	e := newListingTestEnv(t)
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(13)).Return(true, nil)
	e.repo.EXPECT().CategoryAllowsDelivery(gomock.Any(), int32(13)).Return(false, nil)
	_, err := e.uc.Create(context.Background(), authorID, domain.ListingFields{CategoryID: set(int32(13)), HasDelivery: ptr(true)}, false)
	require.ErrorIs(t, err, domain.ErrDeliveryNotAllowed)
	s := domain.ListingState{UserID: authorID, Status: domain.AdStatusDraft, CategoryID: ptr(int32(12)), HasDelivery: true}
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(12)).Return(true, nil)
	e.repo.EXPECT().CategoryAllowsDelivery(gomock.Any(), int32(12)).Return(true, nil)
	expectListingCreate(e, s, false, domain.Listing{ID: 7, Seller: domain.Seller{ID: authorID}})
	_, err = e.uc.Create(context.Background(), authorID, domain.ListingFields{CategoryID: set(int32(12)), HasDelivery: ptr(true)}, false)
	require.NoError(t, err)
	expectListingState(e, s)
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(13)).Return(true, nil)
	e.repo.EXPECT().CategoryAllowsDelivery(gomock.Any(), int32(13)).Return(false, nil)
	_, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{CategoryID: set(int32(13))}, nil)
	require.ErrorIs(t, err, domain.ErrDeliveryNotAllowed)
	expectListingState(e, s)
	e.repo.EXPECT().IsLeafCategory(gomock.Any(), int32(13)).Return(true, nil)
	s.CategoryID = ptr(int32(13))
	s.HasDelivery = false
	e.repo.EXPECT().Update(gomock.Any(), int32(7), s).Return(nil)
	expectListingGet(e, domain.Listing{Seller: domain.Seller{ID: authorID}})
	_, err = e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{CategoryID: set(int32(13)), HasDelivery: ptr(false)}, nil)
	require.NoError(t, err)
}

func TestListing_CloseOnModeration(t *testing.T) {
	e := newListingTestEnv(t)
	expectListingClose(e, domain.AdStatusModeration)
	l, err := e.uc.Close(context.Background(), authorID, 7, "changed_mind")
	require.NoError(t, err)
	require.Equal(t, domain.AdStatusClosed, l.Status)
}

func TestListing_DeleteRemovesPhotos(t *testing.T) {
	e := newListingTestEnv(t)
	expectListingState(e, listingState(domain.AdStatusActive))
	e.repo.EXPECT().ActiveOrder(gomock.Any(), int32(7)).Return(int32(0), false, nil)
	e.repo.EXPECT().Delete(gomock.Any(), int32(7)).Return([]string{ListingPhotoURLPrefix + "a.png", ListingPhotoURLPrefix + "b.png"}, nil)
	e.storage.EXPECT().Delete(gomock.Any(), "a.png").Return(nil)
	e.storage.EXPECT().Delete(gomock.Any(), "b.png").Return(nil)
	require.NoError(t, e.uc.Delete(context.Background(), authorID, 7))
	e.repo.EXPECT().Get(gomock.Any(), int32(7), authorID).Return(domain.Listing{}, domain.ErrListingNotFound)
	_, err := e.uc.Get(context.Background(), 7, authorID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
}

func TestListing_UploadPhotoRejectsNonImage(t *testing.T) {
	e := newListingTestEnv(t)
	_, err := e.uc.UploadPhoto(context.Background(), authorID, []byte("not an image"))
	require.ErrorIs(t, err, domain.ErrUnsupportedImage)
}

func TestListing_MaintenanceDeletesStalePhotos(t *testing.T) {
	e := newListingTestEnv(t)
	data := pngBytes(t)
	var saved string
	e.storage.EXPECT().Save(gomock.Any(), gomock.Any(), data).DoAndReturn(func(_ context.Context, name string, _ []byte) error { saved = name; return nil })
	e.repo.EXPECT().CreatePhoto(gomock.Any(), authorID, gomock.Cond(func(url string) bool { return url == ListingPhotoURLPrefix+saved })).Return(int32(3), nil)
	photo, err := e.uc.UploadPhoto(context.Background(), authorID, data)
	require.NoError(t, err)
	require.Equal(t, int32(3), photo.ID)
	now := time.Now()
	e.repo.EXPECT().ApproveModeration(gomock.Any()).Return(int64(1), nil)
	e.repo.EXPECT().DeleteStalePhotos(gomock.Any(), now.Add(-stalePhotoAge)).Return(nil, nil)
	require.NoError(t, e.uc.RunMaintenance(context.Background(), now))
	later := now.Add(25 * time.Hour)
	e.repo.EXPECT().ApproveModeration(gomock.Any()).Return(int64(0), nil)
	e.repo.EXPECT().DeleteStalePhotos(gomock.Any(), later.Add(-stalePhotoAge)).Return([]string{photo.URL}, nil)
	e.storage.EXPECT().Delete(gomock.Any(), saved).Return(nil)
	require.NoError(t, e.uc.RunMaintenance(context.Background(), later))
}

func TestListing_CategoriesBuildsTree(t *testing.T) {
	e := newListingTestEnv(t)
	parent := int32(1)
	missing := int32(99)
	categories := []domain.Category{
		{ID: 1, Name: "Спорт", Slug: "sport", DeliveryAllowed: true},
		{ID: 2, ParentID: &parent, Name: "Велосипеды", Slug: "bikes"},
		{ID: 3, ParentID: &missing, Name: "Сироты", Slug: "orphans"},
	}

	e.repo.EXPECT().ListCategories(gomock.Any()).Return(categories, nil)
	tree, err := e.uc.Categories(context.Background())
	require.NoError(t, err)
	require.Len(t, tree, 2)
	require.Equal(t, "sport", tree[0].Slug)
	require.True(t, tree[0].DeliveryAllowed)
	require.False(t, tree[0].Children[0].DeliveryAllowed)
	require.Len(t, tree[0].Children, 1)
	require.Equal(t, int32(2), tree[0].Children[0].ID)
	require.NotNil(t, tree[0].Children[0].Children)
	require.Equal(t, "orphans", tree[1].Slug)
}

func TestListing_GetHidesRejectedFromOthers(t *testing.T) {
	e := newListingTestEnv(t)
	l := domain.Listing{Status: domain.AdStatusRejected, Seller: domain.Seller{ID: authorID}}
	e.repo.EXPECT().Get(gomock.Any(), int32(7), otherID).Return(l, nil)
	_, err := e.uc.Get(context.Background(), 7, otherID)
	require.ErrorIs(t, err, domain.ErrListingNotFound)
	expectListingGet(e, l)
	own, err := e.uc.Get(context.Background(), 7, authorID)
	require.NoError(t, err)
	require.Equal(t, l.Status, own.Status)
	require.True(t, own.IsOwner)
}

func TestListing_UpdateRejectedGoesBackToModeration(t *testing.T) {
	e := newListingTestEnv(t)
	s := listingState(domain.AdStatusRejected)
	expectListingState(e, s)
	_, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{PhotoIDs: &[]int32{}}, nil)
	require.ErrorIs(t, err, domain.ErrPhotosRequired)
	expectListingState(e, s)
	s.Title = "Fixed"
	s.Status = domain.AdStatusModeration
	e.repo.EXPECT().Update(gomock.Any(), int32(7), s).Return(nil)
	expectListingGet(e, domain.Listing{Status: s.Status, Title: s.Title, Seller: domain.Seller{ID: authorID}})
	l, err := e.uc.Update(context.Background(), authorID, 7, domain.ListingFields{Title: ptr("Fixed")}, nil)
	require.NoError(t, err)
	require.Equal(t, s.Status, l.Status)
	require.Equal(t, s.Title, l.Title)
}
