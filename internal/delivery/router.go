package delivery

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/ratelimit"
)

const (
	requestTimeout   = 5 * time.Second
	avatarPath       = "/api/me/avatar"
	listingPhotoPath = "/api/listing-photos"
)

func NewRouter(frontendOrigin, uploadDir string, authHandler *AuthHandler, resetHandler *PasswordResetHandler,
	profileHandler *ProfileHandler, adHandler *AdHandler, listingHandler *ListingHandler, csrfHandler *CSRFHandler,
	addressHandler *AddressHandler, cityHandler *CityHandler, sellerHandler *SellerHandler) http.Handler {
	r := mux.NewRouter()
	r.Use(recoverMiddleware, csrfHandler.Middleware)
	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{"status": "ok"})
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/csrf", csrfHandler.Token).Methods(http.MethodGet)

	loginLimit := ratelimit.New(10, time.Minute)
	registerLimit := ratelimit.New(5, 10*time.Minute)
	forgotLimit := ratelimit.New(5, 10*time.Minute)
	passwordLimit := ratelimit.New(10, 10*time.Minute)

	r.HandleFunc("/api/register", rateLimit(registerLimit, authHandler.Register)).Methods(http.MethodPost)
	r.HandleFunc("/api/login", rateLimit(loginLimit, authHandler.Login)).Methods(http.MethodPost)
	r.HandleFunc("/api/logout", authHandler.Logout).Methods(http.MethodPost)
	r.HandleFunc("/api/password/forgot", rateLimit(forgotLimit, resetHandler.Forgot)).Methods(http.MethodPost)
	r.HandleFunc("/api/password/reset", resetHandler.Check).Methods(http.MethodGet)
	r.HandleFunc("/api/password/reset", rateLimit(passwordLimit, resetHandler.Reset)).Methods(http.MethodPost)

	auth := func(h http.HandlerFunc) http.Handler { return authHandler.RequireAuth(h) }
	r.Handle("/api/me", auth(profileHandler.Get)).Methods(http.MethodGet)
	r.Handle("/api/me", auth(profileHandler.Update)).Methods(http.MethodPatch)
	r.Handle("/api/me/password", auth(rateLimit(passwordLimit, profileHandler.ChangePassword))).Methods(http.MethodPut)
	r.Handle(avatarPath, auth(profileHandler.UploadAvatar)).Methods(http.MethodPost)
	r.Handle("/api/me/listings", auth(adHandler.ListMine)).Methods(http.MethodGet)

	const addressPath = "/api/me/addresses/{address_id:[0-9]+}"
	r.Handle("/api/me/addresses", auth(addressHandler.List)).Methods(http.MethodGet)
	r.Handle("/api/me/addresses", auth(addressHandler.Create)).Methods(http.MethodPost)
	r.Handle(addressPath, auth(addressHandler.Update)).Methods(http.MethodPatch)
	r.Handle(addressPath, auth(addressHandler.Delete)).Methods(http.MethodDelete)

	r.HandleFunc("/api/cities", cityHandler.Search).Methods(http.MethodGet)

	const sellerPath = "/api/sellers/{seller_id:[0-9]+}"
	r.HandleFunc(sellerPath, sellerHandler.Get).Methods(http.MethodGet)
	r.HandleFunc(sellerPath+"/listings", sellerHandler.Listings).Methods(http.MethodGet)
	r.HandleFunc(sellerPath+"/reviews", sellerHandler.Reviews).Methods(http.MethodGet)
	r.Handle(sellerPath+"/reports", auth(sellerHandler.Report)).Methods(http.MethodPost)

	r.HandleFunc("/api/categories", listingHandler.Categories).Methods(http.MethodGet)
	r.HandleFunc("/api/listings", adHandler.List).Methods(http.MethodGet)
	r.Handle("/api/listings", auth(listingHandler.Create)).Methods(http.MethodPost)
	r.Handle(listingPhotoPath, auth(listingHandler.UploadPhoto)).Methods(http.MethodPost)

	const listingPath = "/api/listings/{listing_id:[0-9]+}"
	r.Handle(listingPath, authHandler.OptionalAuth(http.HandlerFunc(listingHandler.Get))).Methods(http.MethodGet)
	r.Handle(listingPath, auth(listingHandler.Update)).Methods(http.MethodPatch)
	r.Handle(listingPath, auth(listingHandler.Delete)).Methods(http.MethodDelete)
	r.Handle(listingPath+"/close", auth(listingHandler.Close)).Methods(http.MethodPost)

	r.PathPrefix("/uploads/").Handler(uploads(uploadDir)).Methods(http.MethodGet, http.MethodHead)

	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == avatarPath || req.URL.Path == listingPhotoPath {
			r.ServeHTTP(w, req)
			return
		}
		timed.ServeHTTP(w, req)
	})

	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(handler))))
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
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+requestIDHeader+", "+csrfHeader)
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
