package delivery

import (
	"github.com/gorilla/mux"
	"net/http"
)

func NewSearchRouter(frontendOrigin string) http.Handler {
	r := mux.NewRouter()
	r.Use(recoverMiddleware)
	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)
	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{"status": "ok", "service": "search"})
	}).Methods(http.MethodGet)
	r.HandleFunc("/api/search", func(w http.ResponseWriter, req *http.Request) {
		writeError(req.Context(), w, http.StatusNotImplemented, "search is not implemented")
	}).Methods(http.MethodGet)
	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)
	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(timed))))
}
