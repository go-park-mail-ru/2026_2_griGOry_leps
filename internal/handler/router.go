package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

const requestTimeout = 5 * time.Second

func NewRouter(frontendOrigin string, authHandler *AuthHandler, adHandler *AdHandler) http.Handler {
	r := mux.NewRouter()
	r.Use(recoverMiddleware)
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	api := r.PathPrefix("/api").Subrouter()
	api.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}).Methods(http.MethodGet)

	api.HandleFunc("/register", authHandler.Register).Methods(http.MethodPost)
	api.HandleFunc("/login", authHandler.Login).Methods(http.MethodPost)
	api.HandleFunc("/logout", authHandler.Logout).Methods(http.MethodPost)
	api.HandleFunc("/me", authHandler.Me).Methods(http.MethodGet)

	api.HandleFunc("/ads", adHandler.List).Methods(http.MethodGet)

	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)

	return corsMiddleware(frontendOrigin, jsonContentType(timed))
}

func corsMiddleware(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
		next.ServeHTTP(w, r)
	})
}

func recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
