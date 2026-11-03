package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type testRouter struct {
	http.Handler
	auth   *MockAuthUsecase
	ads    *MockAdUsecase
	seller *MockSellerUsecase
	city   *MockCityUsecase
}

func newTestRouter(t *testing.T) *testRouter { return newTestRouterWithUploads(t, "") }
func newTestRouterWithUploads(t *testing.T, dir string) *testRouter {
	t.Helper()
	ctrl := gomock.NewController(t)
	r := &testRouter{auth: NewMockAuthUsecase(ctrl), ads: NewMockAdUsecase(ctrl), seller: NewMockSellerUsecase(ctrl), city: NewMockCityUsecase(ctrl)}
	r.Handler = NewRouter("http://front.test", dir, NewAuthHandler(r.auth, false), NewPasswordResetHandler(NewMockPasswordResetUsecase(ctrl)),
		NewProfileHandler(NewMockProfileUsecase(ctrl)), NewAdHandler(r.ads), NewListingHandler(NewMockListingUsecase(ctrl)), newCSRFHandler(),
		NewAddressHandler(NewMockAddressUsecase(ctrl)), NewCityHandler(r.city), NewSellerHandler(r.seller),
		NewEmailVerificationHandler(NewMockEmailVerificationUsecase(ctrl)), NewCartHandler(NewMockCartUsecase(ctrl)), NewOrderHandler(NewMockOrderUsecase(ctrl), "http://api.test", "http://front.test"))
	return r
}

func TestRouter_SetsRequestIDHeader(t *testing.T) {
	buf := captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	id := rec.Header().Get(requestIDHeader)
	require.Regexp(t, generatedID, id)
	require.Equal(t, id, accessRecord(t, buf)["request_id"])
}

func TestRouter_PreflightHasRequestID(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodOptions, "/api/login", nil))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.NotEmpty(t, rec.Header().Get(requestIDHeader))
}

func TestRouter_CORSAllowsAndExposesRequestID(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), requestIDHeader)
	require.Equal(t, requestIDHeader+", Retry-After", rec.Header().Get("Access-Control-Expose-Headers"))
}

func TestRouter_PanicLoggedWith500(t *testing.T) {
	buf := captureLogs(t)
	req := httptest.NewRequest(http.MethodGet, "/api/listings", nil)
	req.Header.Set(requestIDHeader, "panic-req")
	rec := httptest.NewRecorder()

	router := newTestRouter(t)
	router.ads.EXPECT().List(gomock.Any(), domain.ListAdsParams{}).DoAndReturn(func(context.Context, domain.ListAdsParams) (domain.AdPage, error) { panic("boom") })
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)

	var panicLogged bool
	for _, record := range logRecords(t, buf) {
		if record["msg"] == "panic recovered" {
			panicLogged = true
			require.Equal(t, "panic-req", record["request_id"])
			require.Equal(t, "boom", record["panic"])
		}
	}
	require.True(t, panicLogged, buf.String())

	access := accessRecord(t, buf)
	require.EqualValues(t, http.StatusInternalServerError, access["status"])
	require.Equal(t, "panic-req", access["request_id"])
}

func TestRouter_LoginFlowWithCSRF(t *testing.T) {
	captureLogs(t)
	router := newTestRouter(t)
	router.auth.EXPECT().Login(gomock.Any(), "a@b.ru", "Secret123").Return(domain.User{ID: 1}, testSession(), nil)

	tokenRec := httptest.NewRecorder()
	router.ServeHTTP(tokenRec, httptest.NewRequest(http.MethodGet, "/api/csrf", nil))
	require.Equal(t, http.StatusOK, tokenRec.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(tokenRec.Body.Bytes(), &resp))

	req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"login":"a@b.ru","password":"Secret123"}`))
	req.Header.Set(csrfHeader, resp["csrf_token"])
	for _, c := range tokenRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRouter_PostWithoutCSRFTokenForbidden(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/logout", nil))

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":"invalid csrf token"}`, rec.Body.String())
}

func TestRouter_SecurityHeaders(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), csrfHeader)
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), idempotencyHeader)
}

func TestRouter_MeRequiresSession(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/me", nil))

	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRouter_ServesUploads(t *testing.T) {
	captureLogs(t)
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "avatars"), 0o755))
	png := []byte("\x89PNG\r\n\x1a\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "avatars", "a.png"), png, 0o644))
	router := newTestRouterWithUploads(t, dir)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/uploads/avatars/a.png", nil))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Header().Get("Cache-Control"), "immutable")
	require.Equal(t, png, rec.Body.Bytes())

	for _, path := range []string{"/uploads/avatars/", "/uploads/avatars/missing.png"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		require.Equal(t, http.StatusNotFound, rec.Code, path)
		require.Empty(t, rec.Header().Get("Cache-Control"), path)
	}
}

func TestRouter_NewRoutesRequireAuth(t *testing.T) {
	captureLogs(t)
	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/me/listings"},
		{http.MethodGet, "/api/me/addresses"},
		{http.MethodPost, "/api/me/addresses"},
		{http.MethodPatch, "/api/me/addresses/1"},
		{http.MethodDelete, "/api/me/addresses/1"},
		{http.MethodPost, "/api/sellers/5/reports"},
		{http.MethodGet, "/api/cart"},
		{http.MethodPost, "/api/checkout/quote"},
		{http.MethodPost, "/api/orders"},
		{http.MethodGet, "/api/orders/1"},
		{http.MethodPost, "/api/orders/1/payment"},
	} {
		token, tokenRec := fetchCSRFToken(t, newCSRFHandler())
		req := httptest.NewRequest(route.method, route.path, nil)
		req.Header.Set(csrfHeader, token)
		req.AddCookie(responseCookie(tokenRec, csrfCookieName))
		rec := httptest.NewRecorder()

		newTestRouter(t).ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code, route.path)
		require.Equal(t, "not authenticated", decodeBody(t, rec)["error"], route.path)
	}
}

func TestRouter_PublicSellerAndCityRoutes(t *testing.T) {
	captureLogs(t)
	for _, path := range []string{"/api/sellers/5", "/api/sellers/5/listings", "/api/sellers/5/reviews", "/api/cities?query=Мос"} {
		rec := httptest.NewRecorder()

		router := newTestRouter(t)
		switch path {
		case "/api/sellers/5":
			router.seller.EXPECT().Profile(gomock.Any(), int32(5)).Return(domain.SellerProfile{Seller: domain.Seller{ID: 5}}, nil)
		case "/api/sellers/5/listings":
			router.seller.EXPECT().Listings(gomock.Any(), domain.SellerAdFilter{SellerID: 5}).Return(domain.SellerAdPage{}, nil)
		case "/api/sellers/5/reviews":
			router.seller.EXPECT().Reviews(gomock.Any(), domain.ReviewFilter{SellerID: 5}).Return(domain.ReviewPage{}, nil)
		default:
			router.city.EXPECT().Search(gomock.Any(), "Мос").Return([]domain.City{}, nil)
		}
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

		require.Equal(t, http.StatusOK, rec.Code, path)
	}

	rec := httptest.NewRecorder()
	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/sellers/abc", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestRouter_AddressIDMustBeNumeric(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	newTestRouter(t).ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/me/addresses/abc", nil))

	require.Equal(t, http.StatusNotFound, rec.Code)
}
