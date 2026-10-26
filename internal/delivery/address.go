package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type AddressHandler struct {
	addresses AddressUsecase
}

func NewAddressHandler(addresses AddressUsecase) *AddressHandler {
	return &AddressHandler{addresses: addresses}
}

type addressRequest struct {
	Street    *string `json:"street"`
	House     *string `json:"house"`
	Comment   *string `json:"comment"`
	IsDefault *bool   `json:"is_default"`
}

func addressResponse(a domain.Address) map[string]any {
	return map[string]any{
		"id":         a.ID,
		"street":     a.Street,
		"house":      a.House,
		"comment":    a.Comment,
		"is_default": a.IsDefault,
	}
}

func decodeAddress(w http.ResponseWriter, r *http.Request) (domain.AddressInput, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req addressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return domain.AddressInput{}, false
	}
	return domain.AddressInput{Street: req.Street, House: req.House, Comment: req.Comment, IsDefault: req.IsDefault}, true
}

func addressErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrMissingStreet), errors.Is(err, domain.ErrStreetTooLong):
		return "street"
	case errors.Is(err, domain.ErrMissingHouse), errors.Is(err, domain.ErrHouseTooLong):
		return "house"
	case errors.Is(err, domain.ErrAddressCommentTooLong):
		return "comment"
	}
	return ""
}

func writeAddressError(w http.ResponseWriter, r *http.Request, action string, err error) {
	if field := addressErrorField(err); field != "" {
		writeFieldError(r.Context(), w, http.StatusBadRequest, field, err.Error())
		return
	}
	if errors.Is(err, domain.ErrAddressNotFound) {
		writeError(r.Context(), w, http.StatusNotFound, err.Error())
		return
	}
	slog.ErrorContext(r.Context(), action, "err", err)
	writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
}

func addressID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, ok := pathID(r, "address_id")
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrAddressNotFound.Error())
	}
	return id, ok
}

func (h *AddressHandler) List(w http.ResponseWriter, r *http.Request) {
	addresses, err := h.addresses.List(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		writeAddressError(w, r, "list addresses", err)
		return
	}

	resp := make([]map[string]any, 0, len(addresses))
	for _, a := range addresses {
		resp = append(resp, addressResponse(a))
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *AddressHandler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeAddress(w, r)
	if !ok {
		return
	}

	a, err := h.addresses.Create(r.Context(), userIDFromContext(r.Context()), in)
	if err != nil {
		writeAddressError(w, r, "create address", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusCreated, addressResponse(a))
}

func (h *AddressHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := addressID(w, r)
	if !ok {
		return
	}
	in, ok := decodeAddress(w, r)
	if !ok {
		return
	}

	a, err := h.addresses.Update(r.Context(), userIDFromContext(r.Context()), id, in)
	if err != nil {
		writeAddressError(w, r, "update address", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, addressResponse(a))
}

func (h *AddressHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := addressID(w, r)
	if !ok {
		return
	}

	if err := h.addresses.Delete(r.Context(), userIDFromContext(r.Context()), id); err != nil {
		writeAddressError(w, r, "delete address", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
