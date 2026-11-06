package delivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestCity_Search(t *testing.T) {
	mock := NewMockCityUsecase(gomock.NewController(t))
	mock.EXPECT().Search(gomock.Any(), "Мос").Return([]domain.City{{ID: 1, Name: "Москва"}}, nil)
	rec := httptest.NewRecorder()

	NewCityHandler(mock).Search(rec, httptest.NewRequest(http.MethodGet, "/api/cities?query=%D0%9C%D0%BE%D1%81", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[{"id": 1, "name": "Москва"}]`, rec.Body.String())
}

func TestCity_Search_Error(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	mock := NewMockCityUsecase(gomock.NewController(t))
	mock.EXPECT().Search(gomock.Any(), "").Return(nil, errors.New("db down"))
	NewCityHandler(mock).Search(rec, httptest.NewRequest(http.MethodGet, "/api/cities", nil))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}
