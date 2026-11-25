package delivery

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const (
	requestTimeout   = 5 * time.Second
	avatarPath       = "/api/me/avatar"
	listingPhotoPath = "/api/listing-photos"
)

func newAPIRouter() *mux.Router {
	router := mux.NewRouter()
	router.Use(captureMetricRoute, recoverMiddleware)
	router.NotFoundHandler = http.HandlerFunc(notFound)
	router.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)
	return router
}

func uploads(dir string) http.Handler {
	root := http.Dir(dir)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f, err := root.Open(strings.TrimPrefix(r.URL.Path, "/uploads"))
		if err != nil {
			notFound(w, r)
			return
		}
		defer func() { _ = f.Close() }()

		info, err := f.Stat()
		if err != nil || info.IsDir() {
			notFound(w, r)
			return
		}

		w.Header().Del("Content-Type")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}

func notFound(w http.ResponseWriter, r *http.Request) {
	writeError(r.Context(), w, http.StatusNotFound, "not found")
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	writeError(r.Context(), w, http.StatusMethodNotAllowed, "method not allowed")
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+requestIDHeader+", "+csrfHeader+", "+idempotencyHeader)
		w.Header().Set("Access-Control-Expose-Headers", requestIDHeader+", Retry-After")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func jsonContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.ErrorContext(r.Context(), "panic recovered", "panic", err)
				writeError(r.Context(), w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
