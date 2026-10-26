package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type sellerStub struct {
	err          error
	adFilter     domain.SellerAdFilter
	reviewFilter domain.ReviewFilter
	report       domain.SellerReport
}

var sellerCreatedAt = time.Date(2022, 4, 10, 10, 0, 0, 0, time.UTC)

func (s *sellerStub) Profile(_ context.Context, id int32) (domain.SellerProfile, error) {
	return domain.SellerProfile{
		SellerShort: domain.SellerShort{
			ID: id, FirstName: "Тимур", Avatar: "t.png", Rating: 4.8, ReviewsCount: 56, CreatedAt: sellerCreatedAt,
		},
		Location: "Москва", PhoneVerified: true, CompletedSalesCount: 140, ActiveListingsCount: 12,
	}, s.err
}

func (s *sellerStub) Listings(_ context.Context, f domain.SellerAdFilter) (domain.SellerAdPage, error) {
	s.adFilter = f
	return domain.SellerAdPage{
		AdPage: domain.AdPage{
			Items: []domain.Ad{{ID: 3, Title: "Велосипед", Price: 1500050, City: "Москва", CategoryID: 14,
				CreatedAt: sellerCreatedAt}},
			Total: 1, Limit: 24,
		},
		Categories: []domain.SellerCategory{{ID: 14, Name: "Спорт и отдых", Count: 1}},
	}, s.err
}

func (s *sellerStub) Reviews(_ context.Context, f domain.ReviewFilter) (domain.ReviewPage, error) {
	s.reviewFilter = f
	return domain.ReviewPage{
		Items: []domain.Review{{
			ID: 1, Author: domain.ReviewAuthor{ID: 9, FirstName: "Анна"}, Rating: 5, Text: "Отлично",
			ListingTitle: "Велосипед", CreatedAt: sellerCreatedAt,
		}},
		Total: 1, Limit: 10,
		ReviewSummary: domain.ReviewSummary{Rating: 5, Distribution: map[int]int{5: 1}},
	}, s.err
}

func (s *sellerStub) Report(_ context.Context, report domain.SellerReport) error {
	s.report = report
	return s.err
}

func sellerRequest(method, target, body string, id string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return mux.SetURLVars(req, map[string]string{"seller_id": id})
}

func TestSeller_Get(t *testing.T) {
	rec := httptest.NewRecorder()

	NewSellerHandler(&sellerStub{}).Get(rec, sellerRequest(http.MethodGet, "/api/sellers/5", "", "5"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"id": 5,
		"first_name": "Тимур",
		"avatar_url": "/uploads/avatars/t.png",
		"rating": 4.8,
		"reviews_count": 56,
		"created_at": "2022-04-10T10:00:00Z",
		"location": "Москва",
		"phone_verified": true,
		"completed_sales_count": 140,
		"active_listings_count": 12
	}`, rec.Body.String())
}

func TestSeller_Get_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NewSellerHandler(&sellerStub{err: domain.ErrSellerNotFound}).Get(rec, sellerRequest(http.MethodGet, "/api/sellers/5", "", "5"))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "seller not found", decodeBody(t, rec)["error"])

	rec = httptest.NewRecorder()
	NewSellerHandler(&sellerStub{}).Get(rec, sellerRequest(http.MethodGet, "/api/sellers/0", "", "0"))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSeller_Listings(t *testing.T) {
	stub := &sellerStub{}
	rec := httptest.NewRecorder()

	NewSellerHandler(stub).Listings(rec, sellerRequest(http.MethodGet,
		"/api/sellers/5/listings?category_id=14&sort=price_asc&limit=60&offset=60", "", "5"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, domain.SellerAdFilter{SellerID: 5, CategoryID: 14, Sort: "price_asc", Limit: 60, Offset: 60}, stub.adFilter)
	require.JSONEq(t, `{
		"items": [{
			"id": 3, "title": "Велосипед", "price": "15000.50", "is_free": false, "image_url": null,
			"city": "Москва", "has_delivery": false, "category_id": 14, "created_at": "2022-04-10T10:00:00Z"
		}],
		"total": 1, "limit": 24, "offset": 0,
		"categories": [{"id": 14, "name": "Спорт и отдых", "count": 1}]
	}`, rec.Body.String())
}

func TestSeller_Listings_BadParams(t *testing.T) {
	for _, query := range []string{"category_id=0", "limit=0", "offset=-1"} {
		rec := httptest.NewRecorder()
		NewSellerHandler(&sellerStub{}).Listings(rec, sellerRequest(http.MethodGet, "/api/sellers/5/listings?"+query, "", "5"))
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}

	rec := httptest.NewRecorder()
	NewSellerHandler(&sellerStub{err: domain.ErrInvalidSort}).Listings(rec, sellerRequest(http.MethodGet, "/api/sellers/5/listings", "", "5"))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSeller_Reviews(t *testing.T) {
	stub := &sellerStub{}
	rec := httptest.NewRecorder()

	NewSellerHandler(stub).Reviews(rec, sellerRequest(http.MethodGet,
		"/api/sellers/5/reviews?sort=rating_asc&with_photos=true&limit=10", "", "5"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, domain.ReviewFilter{SellerID: 5, Sort: "rating_asc", WithPhotos: true, Limit: 10}, stub.reviewFilter)
	require.JSONEq(t, `{
		"items": [{
			"id": 1,
			"author": {"id": 9, "first_name": "Анна", "avatar_url": null},
			"rating": 5,
			"text": "Отлично",
			"listing_title": "Велосипед",
			"photos": [],
			"created_at": "2022-04-10T10:00:00Z"
		}],
		"total": 1, "limit": 10, "offset": 0,
		"rating": 5,
		"distribution": {"1": 0, "2": 0, "3": 0, "4": 0, "5": 1}
	}`, rec.Body.String())
}

func TestSeller_Reviews_BadWithPhotos(t *testing.T) {
	rec := httptest.NewRecorder()
	NewSellerHandler(&sellerStub{}).Reviews(rec, sellerRequest(http.MethodGet, "/api/sellers/5/reviews?with_photos=maybe", "", "5"))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid with_photos", decodeBody(t, rec)["error"])
}

func TestSeller_Report(t *testing.T) {
	stub := &sellerStub{}
	rec := httptest.NewRecorder()

	NewSellerHandler(stub).Report(rec, withAuth(sellerRequest(http.MethodPost, "/api/sellers/5/reports",
		`{"reason":"rude","comment":"Хамил"}`, "5")))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, domain.SellerReport{SellerID: 5, ReporterID: 7, Reason: "rude", Comment: "Хамил"}, stub.report)
}

func TestSeller_Report_Errors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		field  string
	}{
		{domain.ErrInvalidReportReason, http.StatusBadRequest, "reason"},
		{domain.ErrReportCommentTooLong, http.StatusBadRequest, "comment"},
		{domain.ErrSelfReport, http.StatusBadRequest, ""},
		{domain.ErrSellerNotFound, http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		NewSellerHandler(&sellerStub{err: tt.err}).Report(rec, withAuth(sellerRequest(http.MethodPost, "/api/sellers/5/reports",
			`{"reason":"rude"}`, "5")))

		require.Equal(t, tt.status, rec.Code, tt.err)
		body := decodeBody(t, rec)
		require.Equal(t, tt.err.Error(), body["error"])
		if tt.field != "" {
			require.Equal(t, tt.field, body["field"])
		}
	}

	rec := httptest.NewRecorder()
	NewSellerHandler(&sellerStub{}).Report(rec, withAuth(sellerRequest(http.MethodPost, "/api/sellers/5/reports", `{`, "5")))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}
