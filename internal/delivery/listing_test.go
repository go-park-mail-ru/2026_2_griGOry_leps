package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type listingStub struct {
	err        error
	viewerID   int32
	fields     domain.ListingFields
	publish    bool
	publishPtr *bool
	reason     string
}

func (s *listingStub) listing() domain.Listing {
	price := int64(1500050)
	return domain.Listing{
		ID: 7, Title: "Велосипед", Price: &price, Status: domain.AdStatusActive,
		Photos: []domain.ListingPhoto{{ID: 1, URL: "/uploads/listings/a.jpg"}, {ID: 2, URL: "/uploads/listings/b.jpg"}},
		Seller: domain.Seller{ID: 3, FirstName: "Анна", Avatar: "x.png"},
	}
}

func (s *listingStub) Get(_ context.Context, _, viewerID int32) (domain.Listing, error) {
	s.viewerID = viewerID
	return s.listing(), s.err
}

func (s *listingStub) Create(_ context.Context, _ int32, f domain.ListingFields, publish bool) (domain.Listing, error) {
	s.fields, s.publish = f, publish
	return s.listing(), s.err
}

func (s *listingStub) Update(_ context.Context, _, _ int32, f domain.ListingFields, publish *bool) (domain.Listing, error) {
	s.fields, s.publishPtr = f, publish
	return s.listing(), s.err
}

func (s *listingStub) Delete(context.Context, int32, int32) error { return s.err }

func (s *listingStub) Close(_ context.Context, _, _ int32, reason string) (domain.Listing, error) {
	s.reason = reason
	return s.listing(), s.err
}

func (s *listingStub) UploadPhoto(context.Context, int32, []byte) (domain.ListingPhoto, error) {
	return domain.ListingPhoto{ID: 9, URL: "/uploads/listings/c.jpg"}, s.err
}

func (s *listingStub) Categories(context.Context) ([]domain.CategoryNode, error) {
	return []domain.CategoryNode{{ID: 1, Name: "Спорт", Slug: "sport", Children: []domain.CategoryNode{{ID: 2, Name: "Велосипеды", Slug: "bikes"}}}}, s.err
}

func listingRequestFor(method, body string) *http.Request {
	req := unverifiedListingRequest(method, body)
	return req.WithContext(context.WithValue(req.Context(), emailVerifiedKey{}, true))
}

func unverifiedListingRequest(method, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/listings/7", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"listing_id": "7"})
	return req.WithContext(context.WithValue(req.Context(), userIDKey{}, int32(1)))
}

func TestListing_UnverifiedCanOnlyKeepDrafts(t *testing.T) {
	stub := &listingStub{}
	h := NewListingHandler(stub)

	rec := httptest.NewRecorder()
	h.Create(rec, unverifiedListingRequest(http.MethodPost, `{"title":"Диван"}`))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":"email not verified"}`, rec.Body.String())

	rec = httptest.NewRecorder()
	h.Create(rec, unverifiedListingRequest(http.MethodPost, `{"title":"Диван","publish":false}`))
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = httptest.NewRecorder()
	h.Update(rec, unverifiedListingRequest(http.MethodPatch, `{"title":"Диван"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	h.Update(rec, unverifiedListingRequest(http.MethodPatch, `{"publish":true}`))
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestListing_UpdateDistinguishesNullAndMissing(t *testing.T) {
	stub := &listingStub{}
	rec := httptest.NewRecorder()

	NewListingHandler(stub).Update(rec, listingRequestFor(http.MethodPatch,
		`{"title":"Новое","address":null,"price":"100","publish":true}`))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Новое", *stub.fields.Title)
	require.True(t, stub.fields.Address.Set && stub.fields.Address.Null)
	require.Equal(t, domain.Optional[string]{Set: true, Value: "100"}, stub.fields.Price)
	require.False(t, stub.fields.CategoryID.Set)
	require.Nil(t, stub.fields.PhotoIDs)
	require.True(t, *stub.publishPtr)
}

func TestListing_CreatePublishesByDefault(t *testing.T) {
	stub := &listingStub{}
	h := NewListingHandler(stub)

	rec := httptest.NewRecorder()
	h.Create(rec, listingRequestFor(http.MethodPost, `{"title":"Диван","photo_ids":[1,2]}`))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.True(t, stub.publish)
	require.Equal(t, []int32{1, 2}, *stub.fields.PhotoIDs)

	rec = httptest.NewRecorder()
	h.Create(rec, listingRequestFor(http.MethodPost, `{"title":"Диван","publish":false}`))
	require.Equal(t, http.StatusCreated, rec.Code)
	require.False(t, stub.publish)
}

func TestListing_ResponseShape(t *testing.T) {
	rec := httptest.NewRecorder()
	NewListingHandler(&listingStub{}).Get(rec, listingRequestFor(http.MethodGet, ""))

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "15000.50", body["price"])
	require.Nil(t, body["category"])
	require.Nil(t, body["close_reason"])
	require.Equal(t, []any{}, body["attributes"])

	photos := body["photos"].([]any)
	require.Equal(t, true, photos[0].(map[string]any)["is_main"])
	require.Equal(t, false, photos[1].(map[string]any)["is_main"])
	require.Equal(t, "/uploads/avatars/x.png", body["seller"].(map[string]any)["avatar_url"])
}

func TestListing_ErrorStatuses(t *testing.T) {
	cases := []struct {
		err    error
		status int
		field  string
	}{
		{domain.ErrListingNotFound, http.StatusNotFound, ""},
		{domain.ErrForbidden, http.StatusForbidden, ""},
		{domain.ErrListingSold, http.StatusConflict, ""},
		{domain.ErrListingNotDraft, http.StatusConflict, ""},
		{domain.ErrAddressRequired, http.StatusBadRequest, "address"},
		{domain.ErrInvalidPhotoID, http.StatusBadRequest, "photo_ids"},
		{domain.ErrInvalidPrice, http.StatusBadRequest, "price"},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewListingHandler(&listingStub{err: tc.err}).Update(rec, listingRequestFor(http.MethodPatch, `{}`))

			require.Equal(t, tc.status, rec.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, tc.err.Error(), body["error"])
			require.Equal(t, tc.field, body["field"])
		})
	}
}

func TestListing_CloseAndDelete(t *testing.T) {
	stub := &listingStub{}
	h := NewListingHandler(stub)

	rec := httptest.NewRecorder()
	h.Close(rec, listingRequestFor(http.MethodPost, `{"reason":"sold_elsewhere"}`))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "sold_elsewhere", stub.reason)

	rec = httptest.NewRecorder()
	h.Close(rec, listingRequestFor(http.MethodPost, `{`))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = httptest.NewRecorder()
	h.Delete(rec, listingRequestFor(http.MethodDelete, ""))
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListing_BadBodyAndID(t *testing.T) {
	h := NewListingHandler(&listingStub{})

	rec := httptest.NewRecorder()
	h.Update(rec, listingRequestFor(http.MethodPatch, `{"price":15000}`))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/api/listings/99999999999", nil),
		map[string]string{"listing_id": "99999999999"})
	rec = httptest.NewRecorder()
	h.Get(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListing_GetWorksForGuestsAndUsers(t *testing.T) {
	stub := &listingStub{}
	router := mux.NewRouter()
	router.Handle("/api/listings/{listing_id}", newAuthHandler().OptionalAuth(http.HandlerFunc(NewListingHandler(stub).Get)))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/listings/7", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(0), stub.viewerID)

	req := httptest.NewRequest(http.MethodGet, "/api/listings/7", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(1), stub.viewerID)

	guest := mux.NewRouter()
	guest.Handle("/api/listings/{listing_id}", NewAuthHandler(authStub{err: domain.ErrSessionExpired}, false).
		OptionalAuth(http.HandlerFunc(NewListingHandler(stub).Get)))
	rec = httptest.NewRecorder()
	guest.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(0), stub.viewerID)
}

func TestListingHandler_CategoriesAlwaysHasChildrenArray(t *testing.T) {
	rec := httptest.NewRecorder()
	NewListingHandler(&listingStub{}).Categories(rec, httptest.NewRequest(http.MethodGet, "/api/categories", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[{"id":1,"name":"Спорт","slug":"sport","children":[{"id":2,"name":"Велосипеды","slug":"bikes","children":[]}]}]`, rec.Body.String())
}
