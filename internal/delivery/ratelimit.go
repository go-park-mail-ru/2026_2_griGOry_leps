package delivery

import (
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/ratelimit"
)

func rateLimit(l *ratelimit.Limiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		ok, retryAfter := l.Allow(ip, time.Now())
		if !ok {
			setRetryAfter(w, retryAfter)
			slog.WarnContext(r.Context(), "rate limit exceeded", "path", r.URL.Path, "ip", ip)
			writeError(r.Context(), w, http.StatusTooManyRequests, "too many requests")
			return
		}
		next(w, r)
	}
}

func setRetryAfter(w http.ResponseWriter, d time.Duration) {
	w.Header().Set("Retry-After", strconv.Itoa(max(1, int(math.Ceil(d.Seconds())))))
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
