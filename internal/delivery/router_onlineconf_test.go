package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type onlineConfTestRouter struct {
	handler http.Handler
	api     *mux.Router
	checker *MockSessionChecker
	access  *MockOnlineConfAccess
	csrf    *CSRFHandler
}

func newOnlineConfTestRouter(t *testing.T) onlineConfTestRouter {
	t.Helper()
	captureLogs(t)
	ctrl := gomock.NewController(t)
	checker := NewMockSessionChecker(ctrl)
	access := NewMockOnlineConfAccess(ctrl)
	session := NewSessionMiddleware(checker)
	csrf := newCSRFHandler()
	h := NewOnlineConfAccessHandler(access)
	return onlineConfTestRouter{
		NewOnlineConfRouter("http://front.test", session, csrf, h, nil),
		newOnlineConfAPIRouter(session, csrf, h, nil), checker, access, csrf,
	}
}

func onlineConfRequest(h http.Handler, path string, authenticated bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if authenticated {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOnlineConfRouter_Health(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	rec := onlineConfRequest(r.handler, "/api/health", false)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok","service":"onlineconf"}`, rec.Body.String())
}

func TestOnlineConfRouter_CSRF(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	rec := onlineConfRequest(r.handler, "/api/csrf", false)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "csrf_token")
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestOnlineConfRouter_Unauthenticated(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	rec := onlineConfRequest(r.handler, "/api/onlineconf/admin/me", false)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":"not authenticated"}`, rec.Body.String())
	r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{}, domain.ErrSessionExpired)
	rec = onlineConfRequest(r.handler, "/api/onlineconf/admin/me", true)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"error":"not authenticated"}`, rec.Body.String())
}

func TestOnlineConfRouter_AdminGetsMe(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 7}, nil)
	r.access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(nil)
	rec := onlineConfRequest(r.handler, "/api/onlineconf/admin/me", true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"user_id":7}`, rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}

func TestOnlineConfRouter_EntriesRequireSessionAndCSRF(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	path := onlineConfAdminPrefix + "/entries"
	rec := onlineConfRequest(r.handler, path, false)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		requestPath := path
		if method == http.MethodPatch {
			requestPath += "/1"
		}
		req := httptest.NewRequest(method, requestPath, strings.NewReader("{}"))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
		rec = httptest.NewRecorder()
		r.handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusForbidden, rec.Code)
		require.JSONEq(t, `{"error":"invalid csrf token"}`, rec.Body.String())
	}
}

func TestOnlineConfRouter_EveryAdminRouteForbidsRegularUser(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	routes := 0
	err := r.api.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		path, err := route.GetPathTemplate()
		if err != nil || !strings.HasPrefix(path, onlineConfAdminPrefix) {
			return nil
		}
		if route.GetHandler() == nil {
			return nil
		}
		methods, err := route.GetMethods()
		require.NoError(t, err, "admin route %s must declare methods", path)
		for _, method := range methods {
			routes++
			r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 8}, nil)
			r.access.EXPECT().CheckAdmin(gomock.Any(), int32(8)).Return(domain.ErrForbidden)
			cookie := &http.Cookie{Name: sessionCookieName, Value: "s"}
			token, _ := fetchCSRFToken(t, r.csrf, cookie)
			req := httptest.NewRequest(method, path, nil)
			req.AddCookie(cookie)
			req.Header.Set(csrfHeader, token)
			rec := httptest.NewRecorder()
			r.handler.ServeHTTP(rec, req)
			require.Equal(t, http.StatusForbidden, rec.Code, "%s %s", method, path)
			require.JSONEq(t, `{"error":"forbidden"}`, rec.Body.String())
		}
		return nil
	})
	require.NoError(t, err)
	require.Positive(t, routes)
}

func TestOnlineConfRouter_ChecksAdminOnEveryRequest(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 7}, nil).Times(2)
	gomock.InOrder(
		r.access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(nil),
		r.access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(domain.ErrForbidden),
	)
	require.Equal(t, http.StatusOK, onlineConfRequest(r.handler, "/api/onlineconf/admin/me", true).Code)
	require.Equal(t, http.StatusForbidden, onlineConfRequest(r.handler, "/api/onlineconf/admin/me", true).Code)
}

func TestOnlineConfRouter_UnknownAdminPath(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	for _, path := range []string{"/api/onlineconf/admin/nope", "/api/onlineconf/adminx"} {
		for _, authenticated := range []bool{false, true} {
			rec := onlineConfRequest(r.handler, path, authenticated)
			require.Equal(t, http.StatusNotFound, rec.Code, path)
			require.JSONEq(t, `{"error":"not found"}`, rec.Body.String())
		}
	}
}

func TestOnlineConfRouter_ClientCanceled(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	logs := captureLogs(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 7}, nil)
	r.access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).DoAndReturn(func(ctx context.Context, _ int32) error {
		close(entered)
		<-release
		defer close(finished)
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/onlineconf/admin/me", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); r.handler.ServeHTTP(rec, req) }()
	<-entered
	cancel()
	<-done
	close(release)
	<-finished
	require.NotContains(t, rec.Body.String(), "user_id")
	for _, record := range logRecords(t, logs) {
		require.NotEqual(t, "ERROR", record["level"])
	}
}

func TestOnlineConfRouter_AdminResponsesNotCached(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	rec := onlineConfRequest(r.handler, "/api/onlineconf/admin/me", false)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	r.checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 8}, nil)
	r.access.EXPECT().CheckAdmin(gomock.Any(), int32(8)).Return(domain.ErrForbidden)
	rec = onlineConfRequest(r.handler, "/api/onlineconf/admin/me", true)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
}
