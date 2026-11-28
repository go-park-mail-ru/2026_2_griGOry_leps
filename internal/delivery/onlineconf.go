package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
	"github.com/gorilla/mux"
)

type onlineconfService interface {
	Create(context.Context, domain.OnlineconfEntry) (domain.OnlineconfEntry, error)
	Update(context.Context, int, domain.OnlineconfPatch) (domain.OnlineconfEntry, error)
	Search(context.Context, domain.OnlineconfFilter) ([]domain.OnlineconfEntry, error)
}

type onlineconfHandler struct {
	service onlineconfService
}

func NewOnlineconfRouter(service onlineconfService) http.Handler {
	h := &onlineconfHandler{service: service}
	r := mux.NewRouter()
	r.HandleFunc("/v1/entries", h.create).Methods(http.MethodPost)
	r.HandleFunc("/v1/entries/{id}", h.update).Methods(http.MethodPatch)
	r.HandleFunc("/v1/entries", h.search).Methods(http.MethodGet)
	return r
}

func onlineconfBody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return domain.ErrInvalidOnlineconf
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return domain.ErrInvalidOnlineconf
	}
	return nil
}

func onlineconfResponse(w http.ResponseWriter, r *http.Request, status int, data any, err error) {
	if err != nil {
		var code, message string
		switch {
		case errors.Is(err, domain.ErrInvalidOnlineconf):
			status, code, message = 400, "invalid_request", "Недопустимые параметры запроса."
		case errors.Is(err, domain.ErrOnlineconfMissing):
			status, code, message = 404, "entry_not_found", "Запись не найдена."
		case errors.Is(err, domain.ErrOnlineconfExists):
			status, code, message = 409, "entry_conflict", "Имя уже занято в этом скоупе."
		default:
			slog.ErrorContext(r.Context(), "onlineconf request failed", "err", err)
			status, code, message = 500, "internal_error", "Не удалось выполнить операцию."
		}
		data = map[string]any{"error": map[string]string{"code": code, "message": message}}
	}
	writeJSON(r.Context(), w, status, data)
}

func (h *onlineconfHandler) create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Scope string          `json:"scope"`
		Name  string          `json:"name"`
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}
	if err := onlineconfBody(r, &body); err != nil {
		onlineconfResponse(w, r, 0, nil, err)
		return
	}
	entry, err := h.service.Create(r.Context(), domain.OnlineconfEntry{Scope: body.Scope, Name: body.Name, Kind: body.Kind, Value: body.Value})
	onlineconfResponse(w, r, http.StatusCreated, entry, err)
}

func patchString(raw json.RawMessage) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	var s *string
	if err := json.Unmarshal(raw, &s); err != nil || s == nil {
		return nil, domain.ErrInvalidOnlineconf
	}
	return s, nil
}

func (h *onlineconfHandler) update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil || id <= 0 {
		onlineconfResponse(w, r, 0, nil, domain.ErrInvalidOnlineconf)
		return
	}
	var body struct {
		Scope json.RawMessage `json:"scope"`
		Name  json.RawMessage `json:"name"`
		Value json.RawMessage `json:"value"`
	}
	if err := onlineconfBody(r, &body); err != nil {
		onlineconfResponse(w, r, 0, nil, err)
		return
	}
	scope, scopeErr := patchString(body.Scope)
	name, nameErr := patchString(body.Name)
	if scopeErr != nil || nameErr != nil {
		onlineconfResponse(w, r, 0, nil, domain.ErrInvalidOnlineconf)
		return
	}
	entry, err := h.service.Update(r.Context(), int(id), domain.OnlineconfPatch{Scope: scope, Name: name, Value: body.Value})
	onlineconfResponse(w, r, http.StatusOK, entry, err)
}

func (h *onlineconfHandler) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := domain.OnlineconfFilter{Name: query.Get("name")}
	if query.Has("scope") {
		scope := query.Get("scope")
		filter.Scope = &scope
	}
	if query.Has("kind") {
		kind := query.Get("kind")
		filter.Kind = &kind
	}
	entries, err := h.service.Search(r.Context(), filter)
	onlineconfResponse(w, r, http.StatusOK, entries, err)
}
