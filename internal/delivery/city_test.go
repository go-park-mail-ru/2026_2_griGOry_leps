package delivery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type cityStub struct {
	err   error
	query *string
}

func (s cityStub) Search(_ context.Context, query string) ([]domain.City, error) {
	if s.query != nil {
		*s.query = query
	}
	return []domain.City{{ID: 1, Name: "Москва"}}, s.err
}

func TestCity_Search(t *testing.T) {
	var query string
	rec := httptest.NewRecorder()

	NewCityHandler(cityStub{query: &query}).Search(rec, httptest.NewRequest(http.MethodGet, "/api/cities?query=%D0%9C%D0%BE%D1%81", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "Мос", query)
	require.JSONEq(t, `[{"id": 1, "name": "Москва"}]`, rec.Body.String())
}

func TestCity_Search_Error(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	NewCityHandler(cityStub{err: errors.New("db down")}).Search(rec, httptest.NewRequest(http.MethodGet, "/api/cities", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
