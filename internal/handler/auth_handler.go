package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/usecase"
)

const maxRequestBodyBytes = 1 << 20 // 1 MB

type AuthHandler struct {
	auth         *usecase.AuthUsecase
	cookieSecure bool
}

func NewAuthHandler(auth *usecase.AuthUsecase, cookieSecure bool) *AuthHandler {
	return &AuthHandler{auth: auth, cookieSecure: cookieSecure}
}

type registerRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	Nickname  string `json:"nickname"`
	Phone     string `json:"phone"`
}

type loginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func userResponse(user domain.User) map[string]any {
	return map[string]any{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"nickname":   user.Nickname,
		"phone":      user.Phone,
	}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.auth.Register(r.Context(), req.Email, req.Password, req.FirstName, req.Nickname, req.Phone)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidEmail),
			errors.Is(err, usecase.ErrMissingFirstName),
			errors.Is(err, usecase.ErrMissingNickname),
			errors.Is(err, usecase.ErrInvalidPhone),
			errors.Is(err, usecase.ErrWeakPassword):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, usecase.ErrEmailTaken), errors.Is(err, usecase.ErrPhoneTaken):
			writeError(w, http.StatusConflict, err.Error())
		default:
			log.Printf("register error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, userResponse(user))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	session, err := h.auth.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidLogin):
			writeError(w, http.StatusUnauthorized, err.Error())
		default:
			log.Printf("login error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil {
		if err := h.auth.Logout(r.Context(), cookie.Value); err != nil {
			log.Printf("logout error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, err := h.auth.Me(r.Context(), cookie.Value)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrSessionExpired):
			writeError(w, http.StatusUnauthorized, "not authenticated")
		default:
			log.Printf("me error: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, userResponse(user))
}
