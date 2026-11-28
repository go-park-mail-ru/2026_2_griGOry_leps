package delivery

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestRequireAdmin(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		status      int
		body, level string
		called      bool
	}{
		{"allows", nil, 200, "", "", true},
		{"forbidden", fmt.Errorf("access: %w", domain.ErrForbidden), 403, `{"error":"forbidden"}`, "WARN", false},
		{"internal error", errors.New("db down"), 500, `{"error":"internal error"}`, "ERROR", false},
		{"canceled dependency", context.Canceled, 500, `{"error":"internal error"}`, "ERROR", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			access := NewMockOnlineConfAccess(gomock.NewController(t))
			access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(tc.err)
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req = req.WithContext(context.WithValue(req.Context(), userIDKey{}, int32(7)))
			rec := httptest.NewRecorder()
			NewOnlineConfAccessHandler(access).RequireAdmin(next).ServeHTTP(rec, req)
			require.Equal(t, tc.called, called)
			require.Equal(t, tc.status, rec.Code)
			if tc.body != "" {
				require.JSONEq(t, tc.body, rec.Body.String())
			}
			records := logRecords(t, logs)
			if tc.level == "" {
				require.Empty(t, records)
			} else {
				require.Len(t, records, 1)
				require.Equal(t, tc.level, records[0]["level"])
				if tc.level == "WARN" {
					require.Equal(t, float64(7), records[0]["user_id"])
				}
			}
		})
	}
}

func TestRequireAdmin_NoUserInContext(t *testing.T) {
	captureLogs(t)
	access := NewMockOnlineConfAccess(gomock.NewController(t))
	access.EXPECT().CheckAdmin(gomock.Any(), int32(0)).Return(domain.ErrForbidden)
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("handler called") })
	NewOnlineConfAccessHandler(access).RequireAdmin(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
}

func TestRequireAdmin_ClientCanceled(t *testing.T) {
	logs := captureLogs(t)
	access := NewMockOnlineConfAccess(gomock.NewController(t))
	access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(fmt.Errorf("check: %w", context.Canceled))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(context.WithValue(ctx, userIDKey{}, int32(7)))
	rec := httptest.NewRecorder()
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("handler called") })
	NewOnlineConfAccessHandler(access).RequireAdmin(next).ServeHTTP(rec, req)
	for _, record := range logRecords(t, logs) {
		require.NotEqual(t, "ERROR", record["level"])
	}
	require.Empty(t, rec.Body.String())
}

func TestOnlineConfMe(t *testing.T) {
	access := NewMockOnlineConfAccess(gomock.NewController(t))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(context.WithValue(req.Context(), userIDKey{}, int32(7)))
	rec := httptest.NewRecorder()
	NewOnlineConfAccessHandler(access).Me(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"user_id":7}`, rec.Body.String())
}
