package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type FavoritesHandler struct {
	favorites FavoritesUsecase
}

func NewFavoritesHandler(favorites FavoritesUsecase) *FavoritesHandler {
	return &FavoritesHandler{favorites: favorites}
}

func (h *FavoritesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ads, err := h.favorites.List(ctx, userIDFromContext(ctx))
	if err != nil {
		writeFavoritesError(w, r, "list favorites", err)
		return
	}

	items := make([]map[string]any, 0, len(ads))
	for _, ad := range ads {
		card := adResponse(ad)
		card["status"] = ad.Status
		items = append(items, card)
	}

	writeJSON(ctx, w, http.StatusOK, map[string]any{"items": items})
}

func (h *FavoritesHandler) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := listingID(r)
	if !ok {
		writeError(ctx, w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}

	if err := h.favorites.Add(ctx, userIDFromContext(ctx), id); err != nil {
		writeFavoritesError(w, r, "add favorite", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FavoritesHandler) Remove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := listingID(r)
	if !ok {
		writeError(ctx, w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}

	if err := h.favorites.Remove(ctx, userIDFromContext(ctx), id); err != nil {
		writeFavoritesError(w, r, "remove favorite", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeFavoritesError(w http.ResponseWriter, r *http.Request, op string, err error) {
	ctx := r.Context()

	switch {
	case errors.Is(err, domain.ErrListingNotFound):
		writeError(ctx, w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrListingUnavailable):
		writeError(ctx, w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(ctx, op, "err", err)
		writeError(ctx, w, http.StatusInternalServerError, "internal error")
	}
}
