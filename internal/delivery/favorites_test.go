package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestFavoritesListEmpty(t *testing.T) {
	favorites := NewMockFavoritesUsecase(gomock.NewController(t))
	favorites.EXPECT().List(gomock.Any(), int32(7)).Return(nil, nil)

	req := withAuth(httptest.NewRequest(http.MethodGet, "/api/favorites", nil))
	rec := httptest.NewRecorder()

	NewFavoritesHandler(favorites).List(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"items":[]}`, rec.Body.String())
}

func TestFavoritesChange(t *testing.T) {
	tests := []struct {
		name   string
		method string
		err    error
		status int
	}{
		{"добавление", http.MethodPut, nil, http.StatusNoContent},
		{"удаление", http.MethodDelete, nil, http.StatusNoContent},
		{"объявление удалено", http.MethodPut, domain.ErrListingNotFound, http.StatusNotFound},
		{"объявление недоступно", http.MethodPut, domain.ErrListingUnavailable, http.StatusConflict},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			favorites := NewMockFavoritesUsecase(gomock.NewController(t))
			handler := NewFavoritesHandler(favorites)

			req := withAuth(httptest.NewRequest(tc.method, "/api/favorites/42", nil))
			req = mux.SetURLVars(req, map[string]string{"listing_id": "42"})
			rec := httptest.NewRecorder()

			if tc.method == http.MethodPut {
				favorites.EXPECT().Add(gomock.Any(), int32(7), int32(42)).Return(tc.err)
				handler.Add(rec, req)
			} else {
				favorites.EXPECT().Remove(gomock.Any(), int32(7), int32(42)).Return(tc.err)
				handler.Remove(rec, req)
			}

			require.Equal(t, tc.status, rec.Code)
			if tc.status == http.StatusNoContent {
				require.Empty(t, rec.Body.String())
			}
		})
	}
}
