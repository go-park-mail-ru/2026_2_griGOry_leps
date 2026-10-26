package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type PasswordResetHandler struct {
	resets PasswordResetUsecase
}

func NewPasswordResetHandler(resets PasswordResetUsecase) *PasswordResetHandler {
	return &PasswordResetHandler{resets: resets}
}

type forgotPasswordRequest struct {
	Login string `json:"login"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (h *PasswordResetHandler) Forgot(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.resets.RequestReset(r.Context(), req.Login)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, domain.ErrInvalidContact):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "login", err.Error())
	default:
		slog.ErrorContext(r.Context(), "request password reset", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func (h *PasswordResetHandler) Check(w http.ResponseWriter, r *http.Request) {
	email, err := h.resets.CheckToken(r.Context(), r.URL.Query().Get("token"))
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"email": email})
	case errors.Is(err, domain.ErrInvalidResetToken):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "token", err.Error())
	default:
		slog.ErrorContext(r.Context(), "check reset token", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func (h *PasswordResetHandler) Reset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.resets.ResetPassword(r.Context(), req.Token, req.Password)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, domain.ErrInvalidResetToken):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "token", err.Error())
	case errors.Is(err, domain.ErrWeakPassword), errors.Is(err, domain.ErrPasswordTooLong):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "password", err.Error())
	default:
		slog.ErrorContext(r.Context(), "reset password", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}
