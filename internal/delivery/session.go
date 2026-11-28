package delivery

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type SessionMiddleware struct{ checker SessionChecker }

func NewSessionMiddleware(checker SessionChecker) *SessionMiddleware {
	return &SessionMiddleware{checker: checker}
}
func (h *SessionMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeError(r.Context(), w, http.StatusUnauthorized, "not authenticated")
			return
		}

		user, err := h.checker.Me(r.Context(), cookie.Value)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey{}, user.ID)
		ctx = context.WithValue(ctx, emailVerifiedKey{}, user.EmailVerified)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *SessionMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := h.optionalUserID(r)
		if err != nil {
			writeSessionError(w, r, err)
			return
		}
		if id != 0 {
			r = r.WithContext(context.WithValue(r.Context(), userIDKey{}, id))
		}
		next.ServeHTTP(w, r)
	})
}

func (h *SessionMiddleware) optionalUserID(r *http.Request) (int32, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return 0, nil
	}
	user, err := h.checker.Me(r.Context(), cookie.Value)
	if errors.Is(err, domain.ErrSessionExpired) || errors.Is(err, domain.ErrUserNotFound) {
		return 0, nil
	}
	return user.ID, err
}

func writeSessionError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, context.Canceled) && r.Context().Err() != nil {
		return
	}
	status, message := http.StatusInternalServerError, "internal error"
	switch {
	case errors.Is(err, domain.ErrSessionExpired), errors.Is(err, domain.ErrUserNotFound):
		status, message = http.StatusUnauthorized, "not authenticated"
	case errors.Is(err, authclient.ErrUnavailable):
		status, message = http.StatusServiceUnavailable, "authentication service unavailable"
	case errors.Is(err, authclient.ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		status, message = http.StatusGatewayTimeout, "authentication service timeout"
	case errors.Is(err, authclient.ErrBadResponse):
		status, message = http.StatusBadGateway, "invalid authentication service response"
	case errors.Is(err, authclient.ErrInternal):
		status, message = http.StatusInternalServerError, "authentication service internal error"
	}
	if status >= 500 {
		slog.ErrorContext(r.Context(), "check session", "err", err)
	}
	writeError(r.Context(), w, status, message)
}
