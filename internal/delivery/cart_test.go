package delivery

import (
	"net/http"
	"net/http/httptest"
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
		{domain.ErrInvalidListingIDs, http.StatusBadRequest, `{"error":"invalid request body"}`},
		{domain.ErrOwnListing, http.StatusForbidden, `{"error":"cannot add own listing"}`},
		{domain.ErrListingUnavailable, http.StatusConflict, `{"error":"listing is unavailable"}`},
		{domain.ErrListingNotFound, http.StatusNotFound, `{"error":"listing not found"}`},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		writeCartError(rec, httptest.NewRequest(http.MethodPost, "/api/cart/items", nil), "test", tt.err)
		require.Equal(t, tt.code, rec.Code, tt.err.Error())
		require.JSONEq(t, tt.body, rec.Body.String())
	}
}
