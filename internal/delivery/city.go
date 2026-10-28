package delivery

import (
	"log/slog"
	"net/http"
)

type CityHandler struct {
	cities CityUsecase
}

func NewCityHandler(cities CityUsecase) *CityHandler {
	return &CityHandler{cities: cities}
}

func (h *CityHandler) Search(w http.ResponseWriter, r *http.Request) {
	cities, err := h.cities.Search(r.Context(), r.URL.Query().Get("query"))
	if err != nil {
		slog.ErrorContext(r.Context(), "search cities", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]cityResponse, 0, len(cities))
	for _, c := range cities {
		resp = append(resp, cityResponse{ID: c.ID, Name: c.Name})
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}
