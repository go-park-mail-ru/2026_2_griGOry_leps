package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationUsecase interface {
	List(ctx context.Context, actorID int32, cursor string, limit int) (domain.ModerationPage, error)
	Get(ctx context.Context, actorID, id int32) (domain.ModerationSnapshot, error)
	Decide(ctx context.Context, actorID, id int32, revision int64, approve bool, reason string) error
}

type ModerationHandler struct{ moderation ModerationUsecase }

func NewModerationHandler(moderation ModerationUsecase) *ModerationHandler {
	return &ModerationHandler{moderation: moderation}
}

type moderationRequestResponse struct {
	ID          int32     `json:"id"`
	Title       string    `json:"title"`
	Price       *string   `json:"price"`
	RequestedAt time.Time `json:"requested_at"`
}

// requireModerator отвечает 403 и возвращает false, если сессия не принадлежит модератору.
func requireModerator(w http.ResponseWriter, r *http.Request) bool {
	if isModerator(r.Context()) {
		return true
	}
	writeModerationError(w, r, domain.ErrForbidden)
	return false
}

func (h *ModerationHandler) List(w http.ResponseWriter, r *http.Request) {
	if !requireModerator(w, r) {
		return
	}
	q := r.URL.Query()
	limit, err := intParam(q, "limit", 1)
	if err != nil || limit > 60 {
		writeFieldError(r.Context(), w, http.StatusBadRequest, "limit", "limit must be 1-60")
		return
	}
	if limit == 0 {
		limit = 20
	}
	page, err := h.moderation.List(r.Context(), userIDFromContext(r.Context()), q.Get("cursor"), limit)
	if err != nil {
		writeModerationError(w, r, err)
		return
	}
	items := make([]moderationRequestResponse, 0, len(page.Items))
	for _, item := range page.Items {
		resp := moderationRequestResponse{ID: item.ID, Title: item.Title, RequestedAt: item.RequestedAt}
		if item.Price != nil {
			price := formatPrice(*item.Price)
			resp.Price = &price
		}
		items = append(items, resp)
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"items": items, "next_cursor": nullable(page.NextCursor)})
}

func (h *ModerationHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !requireModerator(w, r) {
		return
	}
	id, ok := listingID(r)
	if !ok {
		writeModerationError(w, r, domain.ErrListingNotFound)
		return
	}
	item, err := h.moderation.Get(r.Context(), userIDFromContext(r.Context()), id)
	if err != nil {
		writeModerationError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(r.Context(), w, http.StatusOK, struct {
		Listing     listingResponse `json:"listing"`
		Revision    int64           `json:"revision"`
		RequestedAt time.Time       `json:"requested_at"`
	}{newListingResponse(item.Listing), item.Revision, item.RequestedAt})
}

func (h *ModerationHandler) Decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireModerator(w, r) {
			return
		}
		id, ok := listingID(r)
		if !ok {
			writeModerationError(w, r, domain.ErrListingNotFound)
			return
		}
		var input struct {
			Revision int64  `json:"revision"`
			Reason   string `json:"reason"`
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil || decoder.Decode(new(any)) != io.EOF {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.moderation.Decide(r.Context(), userIDFromContext(r.Context()), id, input.Revision, approve, input.Reason); err != nil {
			writeModerationError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeModerationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidRevision):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "revision", err.Error())
	case errors.Is(err, domain.ErrInvalidRejectionReason):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "reason", err.Error())
	case errors.Is(err, domain.ErrInvalidCursor):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "cursor", err.Error())
	case errors.Is(err, domain.ErrModerationConflict):
		writeError(r.Context(), w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrOwnModeration):
		writeError(r.Context(), w, http.StatusForbidden, err.Error())
	default:
		writeListingError(w, r, "moderation", err)
	}
}
