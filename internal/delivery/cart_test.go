package delivery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestCartErrors(t *testing.T) {
	tests := []struct {
		err  error
		code int
		body string
	}{
		{&domain.ItemsUnavailableError{ListingIDs: []int32{3, 4}}, http.StatusConflict, `{"error":"cart items are unavailable","listing_ids":[3,4]}`},
		{domain.ErrInvalidListingIDs, http.StatusBadRequest, `{"error":"invalid request body"}`},
		{domain.ErrOwnListing, http.StatusForbidden, `{"error":"cannot add own listing"}`},
		{domain.ErrListingUnavailable, http.StatusConflict, `{"error":"listing is unavailable"}`},
		{domain.ErrListingNotFound, http.StatusNotFound, `{"error":"listing not found"}`},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		writeCartError(rec, httptest.NewRequest(http.MethodPost, "/api/checkout/quote", nil), "test", tt.err)
		require.Equal(t, tt.code, rec.Code, tt.err.Error())
		require.JSONEq(t, tt.body, rec.Body.String())
	}
}

func TestCartHandler_RejectsTrailingData(t *testing.T) {
	for _, body := range []string{`{"listing_id":1}garbage`, `{"listing_id":1}{}`} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/cart/items", strings.NewReader(body))
		NewCartHandler(nil).Add(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, body)
	}
}

func TestCartItem_DeletedHasOnlyReason(t *testing.T) {
	item := newCartItem(domain.CartEntry{ListingID: 5, Title: "x", Status: domain.ListingDeleted})
	body, err := json.Marshal(item)
	require.NoError(t, err)
	require.JSONEq(t, `{"listing_id":5,"title":null,"price":null,"image_url":null,"address":null,"delivery_from":null,"availability":"unavailable","unavailable_reason":"deleted"}`, string(body))
}
