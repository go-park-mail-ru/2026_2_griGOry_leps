package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type adStub struct {
	err      error
	myFilter domain.MyAdFilter
	reason   string
	deleted  int32
}

var adTime = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func (s *adStub) List(context.Context, domain.ListAdsParams) (domain.AdPage, error) {
	return domain.AdPage{}, s.err
}

func (s *adStub) ListMine(_ context.Context, f domain.MyAdFilter) (domain.MyAdPage, error) {
	s.myFilter = f
	return domain.MyAdPage{
		AdPage: domain.AdPage{
			Items: []domain.Ad{{
				ID: 1, Title: "Диван", IsFree: true, City: "Москва", CategoryID: 4, ImageURL: "/uploads/listings/1.jpg",
				Status: domain.AdStatusClosed, CloseReason: domain.CloseReasonChangedMind, CreatedAt: adTime, UpdatedAt: adTime,
			}},
			Total: 1, Limit: 60,
		},
		Counts: map[string]int{"all": 1, "closed": 1},
	}, s.err
}

func (s *adStub) Close(_ context.Context, userID, id int32, reason string) (domain.Listing, error) {
	s.reason = reason
	return domain.Listing{
		Ad: domain.Ad{
			ID: id, UserID: userID, Title: "Велосипед", Price: 1500000, City: "Москва",
			Status: domain.AdStatusClosed, CloseReason: reason, CreatedAt: adTime, UpdatedAt: adTime,
		},
		Category: &domain.ListingCategory{ID: 12, Name: "Велосипеды", Path: "Спорт и отдых › Велосипеды"},
		Photos:   []domain.ListingPhoto{{ID: 5, URL: "/uploads/listings/5.jpg"}, {ID: 6, URL: "/uploads/listings/6.jpg"}},
		Seller:   domain.SellerShort{ID: userID, FirstName: "Анна", Rating: 4.5, ReviewsCount: 2, CreatedAt: adTime},
		IsOwner:  true,
	}, s.err
}

func (s *adStub) Delete(_ context.Context, _, id int32) error {
	s.deleted = id
	return s.err
}

func listingRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return withAuth(mux.SetURLVars(req, map[string]string{"listing_id": "10"}))
}

func TestAd_ListMine(t *testing.T) {
	stub := &adStub{}
	rec := httptest.NewRecorder()

	NewAdHandler(stub).ListMine(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me/listings?status=closed&limit=60&offset=60", nil)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, domain.MyAdFilter{UserID: 7, Status: "closed", Limit: 60, Offset: 60}, stub.myFilter)
	require.JSONEq(t, `{
		"items": [{
			"id": 1, "title": "Диван", "price": "0.00", "is_free": true, "image_url": "/uploads/listings/1.jpg",
			"city": "Москва", "has_delivery": false, "category_id": 4, "created_at": "2026-10-01T10:00:00Z",
			"status": "closed", "close_reason": "changed_mind", "updated_at": "2026-10-01T10:00:00Z"
		}],
		"total": 1, "limit": 60, "offset": 0,
		"counts": {"all": 1, "closed": 1}
	}`, rec.Body.String())
}

func TestAd_ListMine_InvalidStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	NewAdHandler(&adStub{err: domain.ErrInvalidStatus}).ListMine(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me/listings", nil)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid status", decodeBody(t, rec)["error"])
}

func TestAd_Close(t *testing.T) {
	stub := &adStub{}
	rec := httptest.NewRecorder()

	NewAdHandler(stub).Close(rec, listingRequest(http.MethodPost, "/api/listings/10/close", `{"reason":"sold_elsewhere"}`))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "sold_elsewhere", stub.reason)
	require.JSONEq(t, `{
		"id": 10,
		"title": "Велосипед",
		"category": {"id": 12, "name": "Велосипеды", "path": "Спорт и отдых › Велосипеды"},
		"description": "",
		"photos": [
			{"id": 5, "url": "/uploads/listings/5.jpg", "thumb_url": "/uploads/listings/5.jpg", "is_main": true},
			{"id": 6, "url": "/uploads/listings/6.jpg", "thumb_url": "/uploads/listings/6.jpg", "is_main": false}
		],
		"price": "15000.00",
		"is_free": false,
		"is_negotiable": false,
		"condition": null,
		"city": "Москва",
		"address": null,
		"has_delivery": false,
		"attributes": [],
		"status": "closed",
		"close_reason": "sold_elsewhere",
		"seller": {
			"id": 7, "first_name": "Анна", "avatar_url": null, "rating": 4.5, "reviews_count": 2,
			"created_at": "2026-10-01T10:00:00Z"
		},
		"is_owner": true,
		"in_cart": false,
		"published_at": null,
		"created_at": "2026-10-01T10:00:00Z",
		"updated_at": "2026-10-01T10:00:00Z"
	}`, rec.Body.String())
}

func TestAd_Close_Errors(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{domain.ErrInvalidCloseReason, http.StatusBadRequest},
		{domain.ErrListingNotFound, http.StatusNotFound},
		{domain.ErrNotListingOwner, http.StatusForbidden},
		{domain.ErrListingNotActive, http.StatusConflict},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		NewAdHandler(&adStub{err: tt.err}).Close(rec, listingRequest(http.MethodPost, "/api/listings/10/close", `{"reason":"x"}`))

		require.Equal(t, tt.status, rec.Code, tt.err)
		require.Equal(t, tt.err.Error(), decodeBody(t, rec)["error"])
	}

	rec := httptest.NewRecorder()
	NewAdHandler(&adStub{}).Close(rec, listingRequest(http.MethodPost, "/api/listings/10/close", `{`))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAd_Delete(t *testing.T) {
	stub := &adStub{}
	rec := httptest.NewRecorder()

	NewAdHandler(stub).Delete(rec, listingRequest(http.MethodDelete, "/api/listings/10", ""))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, int32(10), stub.deleted)

	rec = httptest.NewRecorder()
	NewAdHandler(&adStub{err: domain.ErrNotListingOwner}).Delete(rec, listingRequest(http.MethodDelete, "/api/listings/10", ""))
	require.Equal(t, http.StatusForbidden, rec.Code)
}
