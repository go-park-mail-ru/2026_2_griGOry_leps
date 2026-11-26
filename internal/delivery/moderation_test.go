package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type countingModeration struct{ calls int }

func (m *countingModeration) List(context.Context, int32, string, int) (domain.ModerationPage, error) {
	m.calls++
	return domain.ModerationPage{}, nil
}

func (m *countingModeration) Get(context.Context, int32, int32) (domain.ModerationSnapshot, error) {
	m.calls++
	return domain.ModerationSnapshot{}, nil
}

func (m *countingModeration) Decide(context.Context, int32, int32, int64, bool, string) error {
	m.calls++
	return nil
}

func moderationRequest(method, body string, moderator bool) *http.Request {
	req := httptest.NewRequest(method, "/api/moderation/listings/7", strings.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"listing_id": "7"})
	ctx := context.WithValue(req.Context(), userIDKey{}, int32(1))
	if moderator {
		ctx = context.WithValue(ctx, moderatorKey{}, true)
	}
	return req.WithContext(ctx)
}

func TestModerationHandler_ForbiddenWithoutRole(t *testing.T) {
	uc := &countingModeration{}
	h := NewModerationHandler(uc)
	handlers := map[string]http.HandlerFunc{
		"list":    h.List,
		"get":     h.Get,
		"approve": h.Decide(true),
		"reject":  h.Decide(false),
	}
	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, moderationRequest(http.MethodPost, `{"revision":1,"reason":"x"}`, false))
			require.Equal(t, http.StatusForbidden, rec.Code)
			require.JSONEq(t, `{"error":"`+domain.ErrForbidden.Error()+`"}`, rec.Body.String())
		})
	}
	require.Zero(t, uc.calls)
}

func TestModerationHandler_ModeratorPasses(t *testing.T) {
	uc := &countingModeration{}
	h := NewModerationHandler(uc)

	rec := httptest.NewRecorder()
	h.List(rec, moderationRequest(http.MethodGet, "", true))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	h.Decide(true)(rec, moderationRequest(http.MethodPost, `{"revision":1}`, true))
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, 2, uc.calls)
}
