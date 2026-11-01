package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type CartHandler struct {
	carts CartUsecase
}

func NewCartHandler(carts CartUsecase) *CartHandler {
	return &CartHandler{carts: carts}
}

type cartSellerResponse struct {
	ID        int32   `json:"id"`
	FirstName string  `json:"first_name"`
	AvatarURL *string `json:"avatar_url"`
	Rating    float64 `json:"rating"`
}

type cartItemResponse struct {
	ListingID         int32   `json:"listing_id"`
	Title             *string `json:"title"`
	Price             *string `json:"price"`
	ImageURL          *string `json:"image_url"`
	Address           *string `json:"address"`
	DeliveryFrom      *string `json:"delivery_from"`
	Availability      string  `json:"availability"`
	UnavailableReason *string `json:"unavailable_reason"`
}

type cartGroupResponse struct {
	Seller cartSellerResponse `json:"seller"`
	Items  []cartItemResponse `json:"items"`
}

type cartResponse struct {
	ItemsCount int                 `json:"items_count"`
	Groups     []cartGroupResponse `json:"groups"`
}

type cartCountResponse struct {
	ItemsCount int `json:"items_count"`
}

type cartBulkAddResponse struct {
	ItemsCount int     `json:"items_count"`
	SkippedIDs []int32 `json:"skipped_ids"`
}

type addCartItemRequest struct {
	ListingID int32 `json:"listing_id"`
}

type listingIDsRequest struct {
	ListingIDs []int32 `json:"listing_ids"`
}

func priceString(kopecks int64) *string {
	s := formatPrice(kopecks)
	return &s
}

func newCartSeller(s domain.CartSeller) cartSellerResponse {
	resp := cartSellerResponse{ID: s.ID, FirstName: s.FirstName, Rating: s.Rating}
	if s.Avatar != "" {
		url := avatarURL(s.Avatar)
		resp.AvatarURL = &url
	}
	return resp
}

func newCartItem(e domain.CartEntry) cartItemResponse {
	reason := e.UnavailableReason()
	if reason != "" {
		item := cartItemResponse{ListingID: e.ListingID, Availability: "unavailable", UnavailableReason: &reason}
		title := e.Title
		item.Title, item.ImageURL, item.Address = &title, e.ImageURL, e.Address
		return item
	}

	title := e.Title
	item := cartItemResponse{
		ListingID: e.ListingID, Title: &title, ImageURL: e.ImageURL, Address: e.Address, Availability: "available",
	}
	if e.Price != nil {
		item.Price = priceString(*e.Price)
	}
	if e.HasDelivery {
		item.DeliveryFrom = priceString(domain.PickupPointPrice)
	}
	return item
}

func newCartResponse(c domain.Cart) cartResponse {
	resp := cartResponse{ItemsCount: c.ItemsCount, Groups: make([]cartGroupResponse, 0, len(c.Groups))}
	for _, g := range c.Groups {
		group := cartGroupResponse{Seller: newCartSeller(g.Seller), Items: make([]cartItemResponse, 0, len(g.Items))}
		for _, e := range g.Items {
			group.Items = append(group.Items, newCartItem(e))
		}
		resp.Groups = append(resp.Groups, group)
	}
	return resp
}

func (h *CartHandler) Get(w http.ResponseWriter, r *http.Request) {
	cart, err := h.carts.Get(r.Context(), userIDFromContext(r.Context()))
	if err != nil {
		writeCartError(w, r, "get cart", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newCartResponse(cart))
}

func (h *CartHandler) Add(w http.ResponseWriter, r *http.Request) {
	var req addCartItemRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	count, err := h.carts.Add(r.Context(), userIDFromContext(r.Context()), req.ListingID)
	if err != nil {
		writeCartError(w, r, "add cart item", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, cartCountResponse{ItemsCount: count})
}

func (h *CartHandler) Remove(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["listing_id"], 10, 32)
	if err != nil || id <= 0 {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrListingNotFound.Error())
		return
	}
	count, err := h.carts.Remove(r.Context(), userIDFromContext(r.Context()), []int32{int32(id)})
	if err != nil {
		writeCartError(w, r, "remove cart item", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, cartCountResponse{ItemsCount: count})
}

func (h *CartHandler) BulkRemove(w http.ResponseWriter, r *http.Request) {
	var req listingIDsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	count, err := h.carts.Remove(r.Context(), userIDFromContext(r.Context()), req.ListingIDs)
	if err != nil {
		writeCartError(w, r, "bulk remove cart items", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, cartCountResponse{ItemsCount: count})
}

func (h *CartHandler) BulkAdd(w http.ResponseWriter, r *http.Request) {
	var req listingIDsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	count, skipped, err := h.carts.BulkAdd(r.Context(), userIDFromContext(r.Context()), req.ListingIDs)
	if err != nil {
		writeCartError(w, r, "bulk add cart items", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, cartBulkAddResponse{ItemsCount: count, SkippedIDs: skipped})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeCartError(w http.ResponseWriter, r *http.Request, op string, err error) {
	ctx := r.Context()
	switch {
	case errors.Is(err, domain.ErrInvalidListingIDs):
		writeError(ctx, w, http.StatusBadRequest, "invalid request body")
	case errors.Is(err, domain.ErrListingNotFound):
		writeError(ctx, w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrOwnListing):
		writeError(ctx, w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrListingUnavailable):
		writeError(ctx, w, http.StatusConflict, err.Error())
	default:
		slog.ErrorContext(ctx, op, "err", err)
		writeError(ctx, w, http.StatusInternalServerError, "internal error")
	}
}
