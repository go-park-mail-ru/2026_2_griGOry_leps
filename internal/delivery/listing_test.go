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
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func testListing() domain.Listing {
	price := int64(1500050)
	return domain.Listing{
		ID: 7, Title: "Велосипед", Price: &price, Status: domain.AdStatusActive,
		Photos: []domain.ListingPhoto{{ID: 1, URL: "/uploads/listings/a.jpg"}, {ID: 2, URL: "/uploads/listings/b.jpg"}},
		Seller: domain.Seller{ID: 3, FirstName: "Анна", Avatar: "x.png"},
	}
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
	stub := NewMockListingUsecase(gomock.NewController(t))
	title := "Диван"
	stub.EXPECT().Create(gomock.Any(), int32(1), domain.ListingFields{Title: &title}, false).Return(testListing(), nil)
	stub.EXPECT().Update(gomock.Any(), int32(1), int32(7), domain.ListingFields{Title: &title}, (*bool)(nil)).Return(testListing(), nil)
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
	stub := NewMockListingUsecase(gomock.NewController(t))
	title, publish := "Новое", true
	stub.EXPECT().Update(gomock.Any(), int32(1), int32(7), domain.ListingFields{Title: &title, Address: domain.Optional[string]{Set: true, Null: true}, Price: domain.Optional[string]{Set: true, Value: "100"}}, &publish).Return(testListing(), nil)
	rec := httptest.NewRecorder()

	NewListingHandler(stub).Update(rec, listingRequestFor(http.MethodPatch,
		`{"title":"Новое","address":null,"price":"100","publish":true}`))

	require.Equal(t, http.StatusOK, rec.Code)

}

func TestListing_CreatePublishesByDefault(t *testing.T) {
	stub := NewMockListingUsecase(gomock.NewController(t))
	title := "Диван"
	photos := []int32{1, 2}
	stub.EXPECT().Create(gomock.Any(), int32(1), domain.ListingFields{Title: &title, PhotoIDs: &photos}, true).Return(testListing(), nil)
	stub.EXPECT().Create(gomock.Any(), int32(1), domain.ListingFields{Title: &title}, false).Return(testListing(), nil)
	h := NewListingHandler(stub)

	rec := httptest.NewRecorder()
	h.Create(rec, listingRequestFor(http.MethodPost, `{"title":"Диван","photo_ids":[1,2]}`))
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = httptest.NewRecorder()
	h.Create(rec, listingRequestFor(http.MethodPost, `{"title":"Диван","publish":false}`))
	require.Equal(t, http.StatusCreated, rec.Code)

}

func TestListing_ResponseShape(t *testing.T) {
	rec := httptest.NewRecorder()
	mock := NewMockListingUsecase(gomock.NewController(t))
	mock.EXPECT().Get(gomock.Any(), int32(7), int32(1)).Return(testListing(), nil)
	NewListingHandler(mock).Get(rec, listingRequestFor(http.MethodGet, ""))

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
		{domain.ErrDeliveryNotAllowed, http.StatusBadRequest, "has_delivery"},
	}
	for _, tc := range cases {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()
			mock := NewMockListingUsecase(gomock.NewController(t))
			mock.EXPECT().Update(gomock.Any(), int32(1), int32(7), domain.ListingFields{}, (*bool)(nil)).Return(domain.Listing{}, tc.err)
			NewListingHandler(mock).Update(rec, listingRequestFor(http.MethodPatch, `{}`))

			require.Equal(t, tc.status, rec.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, tc.err.Error(), body["error"])
			require.Equal(t, tc.field, body["field"])
		})
	}
}

func TestListing_CloseAndDelete(t *testing.T) {
	stub := NewMockListingUsecase(gomock.NewController(t))
	stub.EXPECT().Close(gomock.Any(), int32(1), int32(7), "sold_elsewhere").Return(testListing(), nil)
	stub.EXPECT().Delete(gomock.Any(), int32(1), int32(7)).Return(nil)
	h := NewListingHandler(stub)

	rec := httptest.NewRecorder()
	h.Close(rec, listingRequestFor(http.MethodPost, `{"reason":"sold_elsewhere"}`))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	h.Close(rec, listingRequestFor(http.MethodPost, `{`))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	rec = httptest.NewRecorder()
	h.Delete(rec, listingRequestFor(http.MethodDelete, ""))
	require.Equal(t, http.StatusNoContent, rec.Code)
}

func TestListing_BadBodyAndID(t *testing.T) {
	h := NewListingHandler(NewMockListingUsecase(gomock.NewController(t)))

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
	stub := NewMockListingUsecase(gomock.NewController(t))
	stub.EXPECT().Get(gomock.Any(), int32(7), int32(0)).Return(testListing(), nil).Times(2)
	stub.EXPECT().Get(gomock.Any(), int32(7), int32(1)).Return(testListing(), nil)
	auth := NewMockAuthUsecase(gomock.NewController(t))
	auth.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 1}, nil)
	router := mux.NewRouter()
	router.Handle("/api/listings/{listing_id}", NewAuthHandler(auth, false).OptionalAuth(http.HandlerFunc(NewListingHandler(stub).Get)))

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/listings/7", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	req := httptest.NewRequest(http.MethodGet, "/api/listings/7", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	expiredAuth := NewMockAuthUsecase(gomock.NewController(t))
	expiredAuth.EXPECT().Me(gomock.Any(), "s").Return(domain.User{}, domain.ErrSessionExpired)
	guest := mux.NewRouter()
	guest.Handle("/api/listings/{listing_id}", NewAuthHandler(expiredAuth, false).
		OptionalAuth(http.HandlerFunc(NewListingHandler(stub).Get)))
	rec = httptest.NewRecorder()
	guest.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

}

func TestListingHandler_CategoriesAlwaysHasChildrenArray(t *testing.T) {
	rec := httptest.NewRecorder()
	mock := NewMockListingUsecase(gomock.NewController(t))
	mock.EXPECT().Categories(gomock.Any()).Return([]domain.CategoryNode{{ID: 1, Name: "Спорт", Slug: "sport", DeliveryAllowed: true, Children: []domain.CategoryNode{{ID: 2, Name: "Велосипеды", Slug: "bikes"}}}}, nil)
	NewListingHandler(mock).Categories(rec, httptest.NewRequest(http.MethodGet, "/api/categories", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[{"id":1,"name":"Спорт","slug":"sport","delivery_allowed":true,
		"children":[{"id":2,"name":"Велосипеды","slug":"bikes","delivery_allowed":false,"children":[]}]}]`, rec.Body.String())
}
