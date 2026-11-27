package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// frontendCalls — все запросы фронтенда (src/lib/api/*.ts) и сервис, который должен их обработать.
var frontendCalls = []struct{ method, path, service string }{
	{http.MethodGet, "/api/csrf", "auth"},
	{http.MethodGet, "/api/me", "auth"},
	{http.MethodPatch, "/api/me", "auth"},
	{http.MethodPost, "/api/me/avatar", "auth"},
	{http.MethodDelete, "/api/me/email/pending", "auth"},
	{http.MethodPut, "/api/me/password", "auth"},
	{http.MethodPost, "/api/login", "auth"},
	{http.MethodPost, "/api/register", "auth"},
	{http.MethodPost, "/api/logout", "auth"},
	{http.MethodPost, "/api/password/forgot", "auth"},
	{http.MethodGet, "/api/password/reset?token=t", "auth"},
	{http.MethodPost, "/api/password/reset", "auth"},
	{http.MethodPost, "/api/email/verify", "auth"},
	{http.MethodPost, "/api/email/verify/resend", "auth"},
	{http.MethodPost, "/api/email/verify/resend-by-link", "auth"},
	{http.MethodGet, "/api/cities?query=%D0%9C%D0%BE%D1%81", "auth"},
	{http.MethodGet, "/uploads/avatars/7.png", "auth"},

	{http.MethodGet, "/api/me/listings", "ads"},
	{http.MethodGet, "/api/me/addresses", "ads"},
	{http.MethodPost, "/api/me/addresses", "ads"},
	{http.MethodPatch, "/api/me/addresses/3", "ads"},
	{http.MethodDelete, "/api/me/addresses/3", "ads"},
	{http.MethodGet, "/api/categories", "ads"},
	{http.MethodGet, "/api/cart", "ads"},
	{http.MethodPost, "/api/cart/items", "ads"},
	{http.MethodDelete, "/api/cart/items/5", "ads"},
	{http.MethodPost, "/api/cart/items/bulk-delete", "ads"},
	{http.MethodPost, "/api/checkout/quote", "ads"},
	{http.MethodPost, "/api/orders", "ads"},
	{http.MethodGet, "/api/orders/5", "ads"},
	{http.MethodPost, "/api/orders/5/payment", "ads"},
	{http.MethodGet, "/api/sellers/5", "ads"},
	{http.MethodGet, "/api/sellers/5/listings", "ads"},
	{http.MethodGet, "/api/sellers/5/reviews", "ads"},
	{http.MethodPost, "/api/sellers/5/reports", "ads"},
	{http.MethodGet, "/api/listings", "ads"},
	{http.MethodGet, "/api/listings/7", "ads"},
	{http.MethodPost, "/api/listings", "ads"},
	{http.MethodPatch, "/api/listings/7", "ads"},
	{http.MethodDelete, "/api/listings/7", "ads"},
	{http.MethodPost, "/api/listings/7/close", "ads"},
	{http.MethodPost, "/api/listing-photos", "ads"},
	{http.MethodGet, "/api/moderation/listings", "ads"},
	{http.MethodGet, "/api/moderation/listings/7", "ads"},
	{http.MethodPost, "/api/moderation/listings/7/approve", "ads"},
	{http.MethodPost, "/api/moderation/listings/7/reject", "ads"},
	{http.MethodGet, "/uploads/listings/a.jpg", "ads"},
	{http.MethodGet, "/api/favorites", "ads"},
	{http.MethodPut, "/api/favorites/7", "ads"},
	{http.MethodDelete, "/api/favorites/7", "ads"},

	{http.MethodGet, "/api/search?q=bike&limit=24&offset=0", "search"},
}

func TestGatewayRoutesFrontendCalls(t *testing.T) {
	upstream := func(name string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(name)) }))
	}
	auth, ads, search := upstream("auth"), upstream("ads"), upstream("search")
	defer auth.Close()
	defer ads.Close()
	defer search.Close()
	gw, err := New(auth.URL, ads.URL, search.URL, "http://front.test")
	require.NoError(t, err)

	for _, call := range frontendCalls {
		rec := httptest.NewRecorder()
		gw.ServeHTTP(rec, httptest.NewRequest(call.method, call.path, nil))
		require.Equal(t, call.service, rec.Body.String(), call.method+" "+call.path)
	}
}
