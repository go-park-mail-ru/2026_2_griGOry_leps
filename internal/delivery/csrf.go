package delivery

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/csrf"
)

const (
	csrfHeader     = "X-CSRF-Token"
	csrfCookieName = "csrf_id"
)

var (
	errMissingCSRFToken  = errors.New("missing csrf token")
	errMissingCSRFCookie = errors.New("missing session or csrf cookie")
)

// CSRFHandler выдаёт CSRF-токены и проверяет их на изменяющих запросах.
// Токен привязан к cookie сессии, а до входа — к анонимной cookie csrf_id,
// поэтому после входа и выхода токен нужно запросить заново.
type CSRFHandler struct {
	tokens       *csrf.Tokens
	cookieSecure bool
}

func NewCSRFHandler(tokens *csrf.Tokens, cookieSecure bool) *CSRFHandler {
	return &CSRFHandler{tokens: tokens, cookieSecure: cookieSecure}
}

func (h *CSRFHandler) Token(w http.ResponseWriter, r *http.Request) {
	binding := csrfBinding(r)
	if binding == "" {
		anonID := randomHex(32)
		http.SetCookie(w, &http.Cookie{
			Name:     csrfCookieName,
			Value:    anonID,
			Path:     "/",
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
		binding = "anon:" + anonID
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]string{"csrf_token": h.tokens.Generate(binding, time.Now())})
}

func (h *CSRFHandler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		if err := h.verify(r); err != nil {
			slog.WarnContext(r.Context(), "csrf check failed", "reason", err.Error())
			writeError(r.Context(), w, http.StatusForbidden, "invalid csrf token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *CSRFHandler) verify(r *http.Request) error {
	token := r.Header.Get(csrfHeader)
	if token == "" {
		return errMissingCSRFToken
	}

	binding := csrfBinding(r)
	if binding == "" {
		return errMissingCSRFCookie
	}

	return h.tokens.Verify(token, binding, time.Now())
}

func csrfBinding(r *http.Request) string {
	if c, err := r.Cookie(sessionCookieName); err == nil && c.Value != "" {
		return "session:" + c.Value
	}
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return "anon:" + c.Value
	}
	return ""
}
