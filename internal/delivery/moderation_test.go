package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestModerationHandler_Decisions(t *testing.T) {
	for _, tc := range []struct {
		name    string
		approve bool
		body    string
		err     error
		code    int
	}{
		{"approve", true, `{"revision":3}`, nil, 204},
		{"reject", false, `{"revision":3,"reason":"Fix photo"}`, nil, 204},
		{"stale", true, `{"revision":3}`, domain.ErrModerationConflict, 409},
		{"forbidden", true, `{"revision":3}`, domain.ErrForbidden, 403},
		{"missing reason", false, `{"revision":3}`, domain.ErrInvalidRejectionReason, 400},
		{"missing revision", true, `{}`, domain.ErrInvalidRevision, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockModerationUsecase(gomock.NewController(t))
			var input struct {
				Revision int64
				Reason   string
			}
			require.NoError(t, json.Unmarshal([]byte(tc.body), &input))
			mock.EXPECT().Decide(gomock.Any(), int32(1), int32(7), input.Revision, tc.approve, input.Reason).Return(tc.err)
			rec := httptest.NewRecorder()
			NewModerationHandler(mock).Decide(tc.approve)(rec, listingRequestFor(http.MethodPost, tc.body))
			require.Equal(t, tc.code, rec.Code)
		})
	}
}

func TestModerationHandler_InvalidJSON(t *testing.T) {
	mock := NewMockModerationUsecase(gomock.NewController(t))
	for _, body := range []string{`{`, `{"revision":"3"}`, `{"revision":3,"extra":true}`, `{"revision":3} {}`} {
		rec := httptest.NewRecorder()
		NewModerationHandler(mock).Decide(true)(rec, listingRequestFor(http.MethodPost, body))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	}
}

func TestModerationHandler_GetAndList(t *testing.T) {
	mock := NewMockModerationUsecase(gomock.NewController(t))
	h := NewModerationHandler(mock)
	mock.EXPECT().Get(gomock.Any(), int32(1), int32(7)).Return(domain.ModerationListing{Listing: testListing(), Revision: 4}, nil)
	rec := httptest.NewRecorder()
	h.Get(rec, listingRequestFor(http.MethodGet, ""))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"revision":4`)
	require.Contains(t, rec.Body.String(), `"photos":[`)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	mock.EXPECT().List(gomock.Any(), int32(1), int32(0), 20).Return(nil, nil)
	rec = httptest.NewRecorder()
	h.List(rec, listingRequestFor(http.MethodGet, ""))
	require.JSONEq(t, `{"items":[],"limit":20}`, rec.Body.String())
}

func TestModerationRouter_AuthAndCSRF(t *testing.T) {
	router := newTestRouter(t)
	for _, path := range []string{"/api/admin/moderation/listings", "/api/admin/moderation/listings/7"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	}
	for _, action := range []string{"approve", "reject"} {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/moderation/listings/7/"+action, strings.NewReader(`{"revision":1}`))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
	}
}

func TestModerationHandler_Pagination(t *testing.T) {
	mock := NewMockModerationUsecase(gomock.NewController(t))
	h := NewModerationHandler(mock)
	for _, query := range []string{"?after_id=-1", "?after_id=2147483648", "?limit=101", "?limit=0", "?limit=x"} {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/moderation/listings"+query, nil)
		req = req.WithContext(context.WithValue(req.Context(), userIDKey{}, int32(1)))
		rec := httptest.NewRecorder()
		h.List(rec, req)
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}
