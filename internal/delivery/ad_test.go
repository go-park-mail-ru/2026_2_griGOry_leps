package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type adStub struct {
	err      error
	myFilter domain.MyAdFilter
}

var adTime = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

func kopecks(v int64) *int64 { return &v }

func (s *adStub) List(context.Context, domain.ListAdsParams) (domain.AdPage, error) {
	return domain.AdPage{
		Items: []domain.Ad{{ID: 3, Title: "Книги", IsFree: true, Price: kopecks(0), City: "Москва", CategoryID: 15, CreatedAt: adTime}},
		Total: 1, Limit: 24,
	}, s.err
}

func TestAd_ListReturnsIsFree(t *testing.T) {
	rec := httptest.NewRecorder()

	NewAdHandler(&adStub{}).List(rec, httptest.NewRequest(http.MethodGet, "/api/listings", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"items": [{
			"id": 3, "title": "Книги", "price": "0.00", "is_free": true, "image_url": null,
			"city": "Москва", "has_delivery": false, "category_id": 15, "created_at": "2026-10-01T10:00:00Z"
		}],
		"total": 1, "limit": 24, "offset": 0
	}`, rec.Body.String())
}

func (s *adStub) ListMine(_ context.Context, f domain.MyAdFilter) (domain.MyAdPage, error) {
	s.myFilter = f
	return domain.MyAdPage{
		AdPage: domain.AdPage{
			Items: []domain.Ad{
				{
					ID: 1, Title: "Диван", IsFree: true, Price: kopecks(0), City: "Москва", CategoryID: 4,
					ImageURL: "/uploads/listings/1.jpg", Status: domain.AdStatusClosed, CloseReason: "changed_mind",
					CreatedAt: adTime, UpdatedAt: adTime,
				},
				{ID: 2, Title: "Черновик", Status: domain.AdStatusDraft, CreatedAt: adTime, UpdatedAt: adTime},
			},
			Total: 2, Limit: 60,
		},
		Counts: map[string]int{"all": 2, "closed": 1, "draft": 1},
	}, s.err
}

func TestAd_ListMine(t *testing.T) {
	stub := &adStub{}
	rec := httptest.NewRecorder()

	NewAdHandler(stub).ListMine(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me/listings?status=closed&limit=60&offset=60", nil)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, domain.MyAdFilter{UserID: 7, Status: "closed", Limit: 60, Offset: 60}, stub.myFilter)
	require.JSONEq(t, `{
		"items": [{
			"id": 1, "title": "Диван", "price": "0.00", "is_free": true, "image_url": "/uploads/listings/1.jpg",
			"city": "Москва", "has_delivery": false, "category_id": 4, "created_at": "2026-10-01T10:00:00Z",
			"status": "closed", "close_reason": "changed_mind", "updated_at": "2026-10-01T10:00:00Z"
		}, {
			"id": 2, "title": "Черновик", "price": null, "is_free": false, "image_url": null,
			"city": "", "has_delivery": false, "category_id": 0, "created_at": "2026-10-01T10:00:00Z",
			"status": "draft", "close_reason": null, "updated_at": "2026-10-01T10:00:00Z"
		}],
		"total": 2, "limit": 60, "offset": 0,
		"counts": {"all": 2, "closed": 1, "draft": 1}
	}`, rec.Body.String())
}

func TestAd_ListMine_InvalidStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	NewAdHandler(&adStub{err: domain.ErrInvalidStatus}).ListMine(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me/listings", nil)))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid status", decodeBody(t, rec)["error"])
}

func TestAd_ListMine_BadPagination(t *testing.T) {
	for _, query := range []string{"limit=0", "offset=-1", "limit=x"} {
		rec := httptest.NewRecorder()
		NewAdHandler(&adStub{}).ListMine(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me/listings?"+query, nil)))
		require.Equal(t, http.StatusBadRequest, rec.Code, query)
	}
}
