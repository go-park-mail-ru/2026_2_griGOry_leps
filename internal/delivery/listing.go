package delivery

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type ListingHandler struct {
	listings ListingUsecase
}

func NewListingHandler(listings ListingUsecase) *ListingHandler {
	return &ListingHandler{listings: listings}
}

type listingRequest struct {
	CategoryID   domain.Optional[int32]  `json:"category_id"`
	Title        *string                 `json:"title"`
	PhotoIDs     *[]int32                `json:"photo_ids"`
	Description  *string                 `json:"description"`
	Price        domain.Optional[string] `json:"price"`
	IsFree       *bool                   `json:"is_free"`
	IsNegotiable *bool                   `json:"is_negotiable"`
	Condition    domain.Optional[string] `json:"condition"`
	Address      domain.Optional[string] `json:"address"`
	HasDelivery  *bool                   `json:"has_delivery"`
	Publish      *bool                   `json:"publish"`
}

func (req listingRequest) fields() domain.ListingFields {
	return domain.ListingFields{
		CategoryID:   req.CategoryID,
		Title:        req.Title,
		PhotoIDs:     req.PhotoIDs,
		Description:  req.Description,
		Price:        req.Price,
		IsFree:       req.IsFree,
		IsNegotiable: req.IsNegotiable,
		Condition:    req.Condition,
		Address:      req.Address,
		HasDelivery:  req.HasDelivery,
	}
}

type closeListingRequest struct {
	Reason string `json:"reason"`
}

type listingCategoryResponse struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type listingPhotoResponse struct {
	ID       int32  `json:"id"`
	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url"`
	IsMain   bool   `json:"is_main"`
}

type uploadedPhotoResponse struct {
	ID       int32  `json:"id"`
	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url"`
}

type listingAttributeResponse struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

type sellerShortResponse struct {
	ID           int32     `json:"id"`
	FirstName    string    `json:"first_name"`
	AvatarURL    *string   `json:"avatar_url"`
	Rating       float64   `json:"rating"`
	ReviewsCount int       `json:"reviews_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type listingResponse struct {
	ID           int32                      `json:"id"`
	Title        string                     `json:"title"`
	Category     *listingCategoryResponse   `json:"category"`
	Description  string                     `json:"description"`
	Photos       []listingPhotoResponse     `json:"photos"`
	Price        *string                    `json:"price"`
	IsFree       bool                       `json:"is_free"`
	IsNegotiable bool                       `json:"is_negotiable"`
	Condition    *string                    `json:"condition"`
	City         string                     `json:"city"`
	Address      *string                    `json:"address"`
	HasDelivery  bool                       `json:"has_delivery"`
	Attributes   []listingAttributeResponse `json:"attributes"`
	Status       string                     `json:"status"`
	CloseReason  *string                    `json:"close_reason"`
	Seller       sellerShortResponse        `json:"seller"`
	IsOwner      bool                       `json:"is_owner"`
	InCart       bool                       `json:"in_cart"`
	PublishedAt  *time.Time                 `json:"published_at"`
	CreatedAt    time.Time                  `json:"created_at"`
	UpdatedAt    time.Time                  `json:"updated_at"`
}

func formatPrice(kopecks int64) string {
	return fmt.Sprintf("%d.%02d", kopecks/100, kopecks%100)
}

func newListingResponse(l domain.Listing) listingResponse {
	resp := listingResponse{
		ID:           l.ID,
		Title:        l.Title,
		Description:  l.Description,
		Photos:       make([]listingPhotoResponse, 0, len(l.Photos)),
		IsFree:       l.IsFree,
		IsNegotiable: l.IsNegotiable,
		Condition:    l.Condition,
		City:         l.City,
		Address:      l.Address,
		HasDelivery:  l.HasDelivery,
		Attributes:   []listingAttributeResponse{},
		Status:       l.Status,
		CloseReason:  l.CloseReason,
		Seller:       newSellerShortResponse(l.Seller),
		IsOwner:      l.IsOwner,
		InCart:       l.InCart,
		PublishedAt:  l.PublishedAt,
		CreatedAt:    l.CreatedAt,
		UpdatedAt:    l.UpdatedAt,
	}

	if l.Category != nil {
		resp.Category = &listingCategoryResponse{ID: l.Category.ID, Name: l.Category.Name, Path: l.Category.Path}
	}
	for i, p := range l.Photos {
		resp.Photos = append(resp.Photos, listingPhotoResponse{ID: p.ID, URL: p.URL, ThumbURL: p.URL, IsMain: i == 0})
	}
	if l.Price != nil {
		price := formatPrice(*l.Price)
		resp.Price = &price
	}
	return resp
}

func optionalAvatarURL(name string) *string {
	if name == "" {
		return nil
	}
	url := avatarURL(name)
	return &url
}

func newSellerShortResponse(s domain.Seller) sellerShortResponse {
	return sellerShortResponse{
		ID:           s.ID,
		FirstName:    s.FirstName,
		AvatarURL:    optionalAvatarURL(s.Avatar),
		Rating:       s.Rating,
		ReviewsCount: s.ReviewsCount,
		CreatedAt:    s.CreatedAt,
	}
}

type categoryNodeResponse struct {
	ID       int32                  `json:"id"`
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Children []categoryNodeResponse `json:"children"`
}

func newCategoryNodes(nodes []domain.CategoryNode) []categoryNodeResponse {
	resp := make([]categoryNodeResponse, 0, len(nodes))
	for _, n := range nodes {
		resp = append(resp, categoryNodeResponse{ID: n.ID, Name: n.Name, Slug: n.Slug, Children: newCategoryNodes(n.Children)})
	}
	return resp
}

func (h *ListingHandler) Categories(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.listings.Categories(r.Context())
	if err != nil {
		slog.ErrorContext(r.Context(), "list categories", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newCategoryNodes(nodes))
}

func (h *ListingHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := listingID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}

	listing, err := h.listings.Get(r.Context(), id, userIDFromContext(r.Context()))
	if err != nil {
		writeListingError(w, r, "get listing", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newListingResponse(listing))
}

func (h *ListingHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeListingRequest(w, r)
	if !ok {
		return
	}

	publish := req.Publish == nil || *req.Publish
	listing, err := h.listings.Create(r.Context(), userIDFromContext(r.Context()), req.fields(), publish)
	if err != nil {
		writeListingError(w, r, "create listing", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusCreated, newListingResponse(listing))
}

func (h *ListingHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := listingID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}
	req, ok := decodeListingRequest(w, r)
	if !ok {
		return
	}

	listing, err := h.listings.Update(r.Context(), userIDFromContext(r.Context()), id, req.fields(), req.Publish)
	if err != nil {
		writeListingError(w, r, "update listing", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newListingResponse(listing))
}

func (h *ListingHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := listingID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}

	if err := h.listings.Delete(r.Context(), userIDFromContext(r.Context()), id); err != nil {
		writeListingError(w, r, "delete listing", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ListingHandler) Close(w http.ResponseWriter, r *http.Request) {
	id, ok := listingID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req closeListingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	listing, err := h.listings.Close(r.Context(), userIDFromContext(r.Context()), id, req.Reason)
	if err != nil {
		writeListingError(w, r, "close listing", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newListingResponse(listing))
}

func (h *ListingHandler) UploadPhoto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+64<<10)

	data, err := readFilePart(r)
	if err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.As(err, &tooLarge), errors.Is(err, errFileTooLarge):
			writeError(r.Context(), w, http.StatusRequestEntityTooLarge, "file is too large")
		default:
			writeFieldError(r.Context(), w, http.StatusBadRequest, "file", "file is required")
		}
		return
	}

	photo, err := h.listings.UploadPhoto(r.Context(), userIDFromContext(r.Context()), data)
	switch {
	case err == nil:
		writeJSON(r.Context(), w, http.StatusCreated, uploadedPhotoResponse{ID: photo.ID, URL: photo.URL, ThumbURL: photo.URL})
	case errors.Is(err, domain.ErrUnsupportedImage):
		writeError(r.Context(), w, http.StatusUnsupportedMediaType, err.Error())
	case errors.Is(err, domain.ErrInvalidImage), errors.Is(err, domain.ErrImageTooLarge):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "file", err.Error())
	default:
		slog.ErrorContext(r.Context(), "upload listing photo", "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func decodeListingRequest(w http.ResponseWriter, r *http.Request) (listingRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req listingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return listingRequest{}, false
	}
	return req, true
}

func listingID(r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(mux.Vars(r)["listing_id"], 10, 32)
	if err != nil || id <= 0 {
		return 0, false
	}
	return int32(id), true
}

func writeListingError(w http.ResponseWriter, r *http.Request, op string, err error) {
	ctx := r.Context()
	if field := listingErrorField(err); field != "" {
		writeFieldError(ctx, w, http.StatusBadRequest, field, err.Error())
		return
	}

	switch {
	case errors.Is(err, domain.ErrListingNotFound):
		writeError(ctx, w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrForbidden):
		writeError(ctx, w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrListingSold),
		errors.Is(err, domain.ErrListingNotDraft),
		errors.Is(err, domain.ErrListingNotActive):
		writeError(ctx, w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(ctx, op, "err", err)
		writeError(ctx, w, http.StatusInternalServerError, "internal error")
	}
}

func listingErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrInvalidCategory):
		return "category_id"
	case errors.Is(err, domain.ErrInvalidTitle):
		return "title"
	case errors.Is(err, domain.ErrPhotosRequired),
		errors.Is(err, domain.ErrInvalidPhotoID),
		errors.Is(err, domain.ErrTooManyPhotos):
		return "photo_ids"
	case errors.Is(err, domain.ErrDescriptionTooLong):
		return "description"
	case errors.Is(err, domain.ErrInvalidPrice):
		return "price"
	case errors.Is(err, domain.ErrInvalidCondition):
		return "condition"
	case errors.Is(err, domain.ErrAddressRequired),
		errors.Is(err, domain.ErrAddressTooLong):
		return "address"
	case errors.Is(err, domain.ErrInvalidCloseReason):
		return "reason"
	}
	return ""
}
