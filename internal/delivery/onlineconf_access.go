package delivery

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OnlineConfAccessHandler struct {
	access OnlineConfAccess
}

func NewOnlineConfAccessHandler(access OnlineConfAccess) *OnlineConfAccessHandler {
	return &OnlineConfAccessHandler{access: access}
}

func (h *OnlineConfAccessHandler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := userIDFromContext(r.Context())
		if err := h.access.CheckAdmin(r.Context(), id); err != nil {
			switch {
			case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
				return
			case errors.Is(err, domain.ErrForbidden):
				slog.WarnContext(r.Context(), "onlineconf admin access denied", "user_id", id)
				writeError(r.Context(), w, http.StatusForbidden, "forbidden")
			default:
				slog.ErrorContext(r.Context(), "check onlineconf admin", "err", err)
				writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *OnlineConfAccessHandler) Me(w http.ResponseWriter, r *http.Request) {
	writeJSON(r.Context(), w, http.StatusOK, map[string]int32{"user_id": userIDFromContext(r.Context())})
}
