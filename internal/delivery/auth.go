package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	maxRequestBodyBytes = 1 << 20
	sessionCookieName   = "session_id"
)

type AuthHandler struct {
	auth         AuthUsecase
	cookieSecure bool
}

func NewAuthHandler(auth AuthUsecase, cookieSecure bool) *AuthHandler {
	return &AuthHandler{auth: auth, cookieSecure: cookieSecure}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	Phone     string `json:"phone"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func userResponse(user domain.User) map[string]any {
	return map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"phone":      user.Phone,
	}
}

func (h *AuthHandler) setSessionCookie(w http.ResponseWriter, session domain.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, session, err := h.auth.Register(r.Context(), req.Email, req.Password, req.FirstName, req.Phone)
	if err != nil {
		field := registerErrorField(err)
		if field == "" {
			slog.ErrorContext(r.Context(), "register", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
			return
		}

		status := http.StatusBadRequest
		if errors.Is(err, domain.ErrEmailTaken) ||
			errors.Is(err, domain.ErrPhoneTaken) {
			status = http.StatusConflict
		}

		writeFieldError(r.Context(), w, status, field, err.Error())
		return
	}

	h.setSessionCookie(w, session)
	writeJSON(r.Context(), w, http.StatusCreated, userResponse(user))
}

func registerErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidEmail),
		errors.Is(err, domain.ErrEmailTaken):
		return "email"
	case errors.Is(err, domain.ErrMissingFirstName),
		errors.Is(err, domain.ErrInvalidFirstName):
		return "first_name"
	case errors.Is(err, domain.ErrInvalidPhone),
		errors.Is(err, domain.ErrPhoneTaken):
		return "phone"
	case errors.Is(err, domain.ErrWeakPassword),
		errors.Is(err, domain.ErrPasswordTooLong):
		return "password"
	}
	return ""
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, session, err := h.auth.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidLogin):
			writeError(r.Context(), w, http.StatusUnauthorized, err.Error())
		default:
			slog.ErrorContext(r.Context(), "login", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	h.setSessionCookie(w, session)
	writeJSON(r.Context(), w, http.StatusOK, userResponse(user))
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if err := h.auth.Logout(r.Context(), cookie.Value); err != nil {
			slog.ErrorContext(r.Context(), "logout", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		writeError(r.Context(), w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.auth.Me(r.Context(), cookie.Value)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrSessionExpired):
			writeError(r.Context(), w, http.StatusUnauthorized, "not authenticated")
		default:
			slog.ErrorContext(r.Context(), "me", "err", err)
			writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(r.Context(), w, http.StatusOK, userResponse(user))
}
