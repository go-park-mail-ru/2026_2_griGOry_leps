package delivery

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ModerationHandler struct{ moderation ModerationUsecase }

func NewModerationHandler(moderation ModerationUsecase) *ModerationHandler {
	return &ModerationHandler{moderation: moderation}
}

type moderationItemResponse struct {
	ID        int32     `json:"id"`
	Title     string    `json:"title"`
	Revision  int64     `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *ModerationHandler) List(w http.ResponseWriter, r *http.Request) {
	afterID, err := intParam(r.URL.Query(), "after_id", 0)
	if err != nil || afterID > math.MaxInt32 {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid after_id")
		return
	}
	limit, err := intParam(r.URL.Query(), "limit", 1)
	if err != nil || limit > 100 {
		writeError(r.Context(), w, http.StatusBadRequest, "limit must be 1-100")
		return
	}
	if limit == 0 {
		limit = 20
	}
	items, err := h.moderation.List(r.Context(), userIDFromContext(r.Context()), int32(afterID), limit)
	if err != nil {
		writeModerationError(w, r, err)
		return
	}
	response := make([]moderationItemResponse, 0, len(items))
	for _, item := range items {
		response = append(response, moderationItemResponse{ID: item.ID, Title: item.Title, Revision: item.Revision, UpdatedAt: item.UpdatedAt})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(r.Context(), w, http.StatusOK, map[string]any{"items": response, "limit": limit})
}

func (h *ModerationHandler) Get(w http.ResponseWriter, r *http.Request) {
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
		Listing  listingResponse `json:"listing"`
		Revision int64           `json:"revision"`
	}{Listing: newListingResponse(item.Listing), Revision: item.Revision})
}

func (h *ModerationHandler) Decide(approve bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if err := decoder.Decode(&input); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := decoder.Decode(new(any)); err != io.EOF {
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
	case errors.Is(err, domain.ErrModerationConflict):
		writeError(r.Context(), w, http.StatusConflict, err.Error())
	default:
		writeListingError(w, r, "moderation", err)
	}
}
