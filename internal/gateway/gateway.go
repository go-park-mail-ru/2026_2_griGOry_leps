package gateway

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/logger"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/clientip"
)

func IsAuthPath(p string) bool {
	switch p {
	case "/api/register", "/api/login", "/api/logout", "/api/csrf", "/api/me", "/api/me/password", "/api/me/avatar":
		return true
	}
	return strings.HasPrefix(p, "/api/password/") || strings.HasPrefix(p, "/api/email/") || strings.HasPrefix(p, "/api/me/email/") || strings.HasPrefix(p, "/uploads/avatars/")
}

func New(authURL, adsURL, searchURL, origin string) (http.Handler, error) {
	auth, err := newProxy(authURL)
	if err != nil {
		return nil, err
	}
	ads, err := newProxy(adsURL)
	if err != nil {
		return nil, err
	}
	search, err := newProxy(searchURL)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := logger.NormalizeRequestID(r.Header.Get(logger.RequestIDHeader))
		r = r.Clone(logger.WithRequestID(r.Context(), id))
		r.Header.Set(logger.RequestIDHeader, id)
		w.Header().Set(logger.RequestIDHeader, id)
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-Id, X-CSRF-Token, Idempotency-Key")
		w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id, Retry-After")
		p := path.Clean(r.URL.Path)
		if p == "/internal" || strings.HasPrefix(p, "/internal/") {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		switch {
		case IsAuthPath(p):
			auth.ServeHTTP(w, r)
		case p == "/api/search":
			search.ServeHTTP(w, r)
		default:
			ads.ServeHTTP(w, r)
		}
	}), nil
}

func newProxy(rawURL string) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, errors.New("invalid upstream URL")
	}
	u.Path = ""
	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Director = nil
	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(u)
		req.Out.Host = req.In.Host
		req.SetXForwarded()
		req.Out.Header.Set(logger.RequestIDHeader, logger.RequestID(req.In.Context()))
		req.Out.Header.Set("X-Forwarded-For", clientip.FromRequest(req.In))
		req.Out.Header.Del("X-Real-IP")
		req.Out.Header.Del("Forwarded")
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Keep the ID set at the gateway, without a duplicate upstream header.
		resp.Header.Del(logger.RequestIDHeader)
		for name := range resp.Header {
			if strings.HasPrefix(strings.ToLower(name), "access-control-") {
				resp.Header.Del(name)
			}
		}
		return nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 15 * time.Second
	proxy.Transport = transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if r.Context().Err() != nil {
			return
		}
		slog.ErrorContext(r.Context(), "upstream request failed", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
	}
	return proxy, nil
}
