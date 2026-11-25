package delivery

import (
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/platform/metrics"
	"github.com/gorilla/mux"
)

func withHTTPMetrics(router *mux.Router, next http.Handler) http.Handler {
	return metrics.RecordHTTPRequest(next, func(r *http.Request) string {
		var match mux.RouteMatch
		if router.Match(r, &match) && match.Route != nil {
			if path, err := match.Route.GetPathTemplate(); err == nil {
				return path
			}
		}
		return "unmatched"
	})
}
