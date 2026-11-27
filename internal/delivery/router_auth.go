package delivery

import (
	"net/http"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/ratelimit"
)

func NewAuthRouter(frontendOrigin, uploadDir string, authHandler *AuthHandler, csrfHandler *CSRFHandler, profileHandler *ProfileHandler, resetHandler *PasswordResetHandler, verifyHandler *EmailVerificationHandler, cityHandler *CityHandler) http.Handler {
	session := NewSessionMiddleware(authHandler.auth)
	r := newAPIRouter()
	r.Use(csrfHandler.Middleware)

	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{"status": "ok", "service": "auth"})
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/csrf", csrfHandler.Token).Methods(http.MethodGet)

	loginLimit := ratelimit.New(10, time.Minute)
	registerLimit := ratelimit.New(5, 10*time.Minute)
	forgotLimit := ratelimit.New(5, 10*time.Minute)
	passwordLimit := ratelimit.New(10, 10*time.Minute)
	verifyLimit := ratelimit.New(10, 10*time.Minute)
	resendLimit := ratelimit.New(5, 10*time.Minute)
	profileLimit := ratelimit.New(20, 10*time.Minute)

	r.HandleFunc("/api/register", rateLimit(registerLimit, authHandler.Register)).Methods(http.MethodPost)
	r.HandleFunc("/api/login", rateLimit(loginLimit, authHandler.Login)).Methods(http.MethodPost)
	r.HandleFunc("/api/logout", authHandler.Logout).Methods(http.MethodPost)
	r.HandleFunc("/api/password/forgot", rateLimit(forgotLimit, resetHandler.Forgot)).Methods(http.MethodPost)
	r.HandleFunc("/api/password/reset", resetHandler.Check).Methods(http.MethodGet)
	r.HandleFunc("/api/password/reset", rateLimit(passwordLimit, resetHandler.Reset)).Methods(http.MethodPost)

	auth := func(h http.HandlerFunc) http.Handler { return session.RequireAuth(h) }

	r.HandleFunc("/api/email/verify", rateLimit(verifyLimit, verifyHandler.Verify)).Methods(http.MethodPost)
	r.Handle("/api/email/verify/resend", auth(rateLimit(resendLimit, verifyHandler.Resend))).Methods(http.MethodPost)
	r.HandleFunc("/api/email/verify/resend-by-link", rateLimit(resendLimit, verifyHandler.ResendByToken)).Methods(http.MethodPost)
	r.Handle("/api/me/email/pending", auth(verifyHandler.CancelEmailChange)).Methods(http.MethodDelete)

	r.HandleFunc("/api/cities", cityHandler.Search).Methods(http.MethodGet)

	r.Handle("/api/me", auth(profileHandler.Get)).Methods(http.MethodGet)
	r.Handle("/api/me", auth(rateLimit(profileLimit, profileHandler.Update))).Methods(http.MethodPatch)
	r.Handle("/api/me/password", auth(rateLimit(passwordLimit, profileHandler.ChangePassword))).Methods(http.MethodPut)
	r.Handle(avatarPath, auth(profileHandler.UploadAvatar)).Methods(http.MethodPost)
	r.PathPrefix("/uploads/avatars/").Handler(uploads(uploadDir)).Methods(http.MethodGet, http.MethodHead)
	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == avatarPath {
			r.ServeHTTP(w, req)
			return
		}
		timed.ServeHTTP(w, req)
	})

	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(handler))))
}
