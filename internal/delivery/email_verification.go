package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type EmailVerificationHandler struct {
	verifications EmailVerificationUsecase
}

func NewEmailVerificationHandler(verifications EmailVerificationUsecase) *EmailVerificationHandler {
	return &EmailVerificationHandler{verifications: verifications}
}

type verifyEmailRequest struct {
	Token string `json:"token"`
}

func decodeToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req verifyEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return "", false
	}
	return req.Token, true
}

func (h *EmailVerificationHandler) Verify(w http.ResponseWriter, r *http.Request) {
	token, ok := decodeToken(w, r)
	if !ok {
		return
	}

	result, err := h.verifications.Verify(r.Context(), token)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]any{"email": result.Email, "changed": result.Changed})
	case errors.Is(err, domain.ErrInvalidVerifyToken):
		body := map[string]string{"error": err.Error(), "field": "token"}
		if result.Email != "" {
			body["email"] = result.Email
		}
		writeJSON(r.Context(), w, http.StatusBadRequest, body)
	case errors.Is(err, domain.ErrEmailAlreadyVerified):
		writeError(r.Context(), w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrEmailTaken):
		writeFieldError(r.Context(), w, http.StatusConflict, "email", err.Error())
	default:
		slog.ErrorContext(r.Context(), "verify email", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func (h *EmailVerificationHandler) Resend(w http.ResponseWriter, r *http.Request) {
	email, wait, err := h.verifications.Resend(r.Context(), userIDFromContext(r.Context()))
	writeResendResult(w, r, email, wait, err)
}

func (h *EmailVerificationHandler) ResendByToken(w http.ResponseWriter, r *http.Request) {
	token, ok := decodeToken(w, r)
	if !ok {
		return
	}

	email, wait, err := h.verifications.ResendByToken(r.Context(), token)
	if errors.Is(err, domain.ErrInvalidVerifyToken) {
		writeFieldError(r.Context(), w, http.StatusBadRequest, "token", err.Error())
		return
	}
	writeResendResult(w, r, email, wait, err)
}

func writeResendResult(w http.ResponseWriter, r *http.Request, email string, wait time.Duration, err error) {
	switch {
	case errors.Is(err, domain.ErrEmailAlreadyVerified):
		writeError(r.Context(), w, http.StatusConflict, err.Error())
	case err != nil:
		slog.ErrorContext(r.Context(), "resend email verification", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	case wait > 0:
		setRetryAfter(w, wait)
		writeError(r.Context(), w, http.StatusTooManyRequests, "too many requests")
	default:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"email": email})
	}
}

func (h *EmailVerificationHandler) CancelEmailChange(w http.ResponseWriter, r *http.Request) {
	err := h.verifications.CancelEmailChange(r.Context(), userIDFromContext(r.Context()))
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
	case errors.Is(err, domain.ErrNoPendingEmail):
		writeError(r.Context(), w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(r.Context(), "cancel email change", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}
