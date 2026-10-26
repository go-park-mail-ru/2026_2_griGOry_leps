package delivery

import (
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type SellerHandler struct {
	sellers SellerUsecase
}

func NewSellerHandler(sellers SellerUsecase) *SellerHandler {
	return &SellerHandler{sellers: sellers}
}

func avatarOrNil(name string) any {
	if name == "" {
		return nil
	}
	return avatarURL(name)
}

func sellerShortResponse(s domain.SellerShort) map[string]any {
	return map[string]any{
		"id":            s.ID,
		"first_name":    s.FirstName,
		"avatar_url":    avatarOrNil(s.Avatar),
		"rating":        s.Rating,
		"reviews_count": s.ReviewsCount,
		"created_at":    s.CreatedAt,
	}
}

func sellerProfileResponse(s domain.SellerProfile) map[string]any {
	resp := sellerShortResponse(s.SellerShort)
	resp["location"] = s.Location
	resp["phone_verified"] = s.PhoneVerified
	resp["completed_sales_count"] = s.CompletedSalesCount
	resp["active_listings_count"] = s.ActiveListingsCount
	return resp
}

func reviewResponse(rv domain.Review) map[string]any {
	photos := rv.Photos
	if photos == nil {
		photos = []string{}
	}
	return map[string]any{
		"id": rv.ID,
		"author": map[string]any{
			"id":         rv.Author.ID,
			"first_name": rv.Author.FirstName,
			"avatar_url": avatarOrNil(rv.Author.Avatar),
		},
		"rating":        rv.Rating,
		"text":          rv.Text,
		"listing_title": nullable(rv.ListingTitle),
		"photos":        photos,
		"created_at":    rv.CreatedAt,
	}
}

func sellerID(w http.ResponseWriter, r *http.Request) (int32, bool) {
	id, ok := pathID(r, "seller_id")
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrSellerNotFound.Error())
	}
	return id, ok
}

func writeSellerError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, domain.ErrSellerNotFound):
		writeError(r.Context(), w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidSort):
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
	default:
		slog.ErrorContext(r.Context(), action, "err", err)
		writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
	}
}

func (h *SellerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := sellerID(w, r)
	if !ok {
		return
	}

	seller, err := h.sellers.Profile(r.Context(), id)
	if err != nil {
		writeSellerError(w, r, "get seller", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, sellerProfileResponse(seller))
}

func (h *SellerHandler) Listings(w http.ResponseWriter, r *http.Request) {
	id, ok := sellerID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	f := domain.SellerAdFilter{SellerID: id, Sort: q.Get("sort")}

	categoryID, err := intParam(q, "category_id", 1)
	if err != nil || categoryID > math.MaxInt32 {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid category_id")
		return
	}
	f.CategoryID = int32(categoryID)

	if f.Limit, err = intParam(q, "limit", 1); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	if f.Offset, err = intParam(q, "offset", 0); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.sellers.Listings(r.Context(), f)
	if err != nil {
		writeSellerError(w, r, "list seller ads", err)
		return
	}

	categories := make([]map[string]any, 0, len(page.Categories))
	for _, c := range page.Categories {
		categories = append(categories, map[string]any{"id": c.ID, "name": c.Name, "count": c.Count})
	}
	resp := adPageResponse(page.AdPage, adResponse)
	resp["categories"] = categories
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (h *SellerHandler) Reviews(w http.ResponseWriter, r *http.Request) {
	id, ok := sellerID(w, r)
	if !ok {
		return
	}

	q := r.URL.Query()
	f := domain.ReviewFilter{SellerID: id, Sort: q.Get("sort")}

	var err error
	if v := q.Get("with_photos"); v != "" {
		if f.WithPhotos, err = strconv.ParseBool(v); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "invalid with_photos")
			return
		}
	}
	if f.Limit, err = intParam(q, "limit", 1); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}
	if f.Offset, err = intParam(q, "offset", 0); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.sellers.Reviews(r.Context(), f)
	if err != nil {
		writeSellerError(w, r, "list seller reviews", err)
		return
	}

	items := make([]map[string]any, 0, len(page.Items))
	for _, rv := range page.Items {
		items = append(items, reviewResponse(rv))
	}
	distribution := make(map[string]int, 5)
	for stars := 1; stars <= 5; stars++ {
		distribution[strconv.Itoa(stars)] = page.Distribution[stars]
	}

	writeJSON(r.Context(), w, http.StatusOK, map[string]any{
		"items":        items,
		"total":        page.Total,
		"limit":        page.Limit,
		"offset":       page.Offset,
		"rating":       page.Rating,
		"distribution": distribution,
	})
}

type reportSellerRequest struct {
	Reason  string `json:"reason"`
	Comment string `json:"comment"`
}

func (h *SellerHandler) Report(w http.ResponseWriter, r *http.Request) {
	id, ok := sellerID(w, r)
	if !ok {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	var req reportSellerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(r.Context(), w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.sellers.Report(r.Context(), domain.SellerReport{
		SellerID:   id,
		ReporterID: userIDFromContext(r.Context()),
		Reason:     req.Reason,
		Comment:    req.Comment,
	})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, domain.ErrInvalidReportReason):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "reason", err.Error())
	case errors.Is(err, domain.ErrReportCommentTooLong):
		writeFieldError(r.Context(), w, http.StatusBadRequest, "comment", err.Error())
	case errors.Is(err, domain.ErrSelfReport):
		writeError(r.Context(), w, http.StatusBadRequest, err.Error())
	default:
		writeSellerError(w, r, "report seller", err)
	}
}
