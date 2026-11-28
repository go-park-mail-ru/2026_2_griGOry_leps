package delivery

import (
	"net/http"

	"github.com/gorilla/mux"
)

const onlineConfAdminPrefix = "/api/onlineconf/admin"

func NewOnlineConfRouter(frontendOrigin string, session *SessionMiddleware, csrfHandler *CSRFHandler, access *OnlineConfAccessHandler, entries onlineconfService) http.Handler {
	r := newOnlineConfAPIRouter(session, csrfHandler, access, entries)
	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)
	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(timed))))
}

func newOnlineConfAPIRouter(session *SessionMiddleware, csrfHandler *CSRFHandler, access *OnlineConfAccessHandler, entries onlineconfService) *mux.Router {
	r := newAPIRouter()
	r.Use(csrfHandler.Middleware)
	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{"status": "ok", "service": "onlineconf"})
	}).Methods(http.MethodGet)
	r.HandleFunc("/api/csrf", csrfHandler.Token).Methods(http.MethodGet)

	admin := r.PathPrefix(onlineConfAdminPrefix).Subrouter()
	admin.Use(noStore, session.RequireAuth, access.RequireAdmin)
	admin.HandleFunc("/me", access.Me).Methods(http.MethodGet)
	h := &onlineconfHandler{service: entries}
	admin.HandleFunc("/entries/{id}/history", h.history).Methods(http.MethodGet)
	admin.HandleFunc("/entries", h.search).Methods(http.MethodGet)
	admin.HandleFunc("/entries", h.create).Methods(http.MethodPost)
	admin.HandleFunc("/entries/{id}", h.update).Methods(http.MethodPatch)
	admin.HandleFunc("/entries/{id}", h.delete).Methods(http.MethodDelete)
	admin.NotFoundHandler = http.HandlerFunc(notFound)
	return r
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
