package delivery

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestWriteUsersError(t *testing.T) {
	for name, tc := range map[string]struct {
		err    error
		status int
		body   string
	}{
		"unavailable":  {authclient.ErrUnavailable, http.StatusServiceUnavailable, `{"error":"users service unavailable"}`},
		"timeout":      {authclient.ErrTimeout, http.StatusGatewayTimeout, `{"error":"users service timeout"}`},
		"bad response": {authclient.ErrBadResponse, http.StatusBadGateway, `{"error":"invalid auth service response"}`},
		"internal":     {authclient.ErrInternal, http.StatusBadGateway, `{"error":"invalid auth service response"}`},
		"user missing": {domain.ErrUserMissing, http.StatusBadGateway, `{"error":"invalid auth service response"}`},
	} {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handled := writeUsersError(rec, httptest.NewRequest(http.MethodGet, "/", nil), tc.err)
			require.True(t, handled)
			require.Equal(t, tc.status, rec.Code)
			require.JSONEq(t, tc.body, rec.Body.String())
		})
	}
}

func TestWriteUsersError_IgnoresOtherErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	require.False(t, writeUsersError(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("db down")))
	require.Empty(t, rec.Body.String())
}

func TestListingGet_UsersUnavailableIs503(t *testing.T) {
	rec := httptest.NewRecorder()
	writeListingError(rec, httptest.NewRequest(http.MethodGet, "/", nil), "get listing", authclient.ErrUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestCartAndSellerErrors_UsersUnavailableIs503(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCartError(rec, httptest.NewRequest(http.MethodGet, "/", nil), "get cart", authclient.ErrUnavailable)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	rec = httptest.NewRecorder()
	writeSellerError(rec, httptest.NewRequest(http.MethodGet, "/", nil), "get seller", authclient.ErrTimeout)
	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
}
