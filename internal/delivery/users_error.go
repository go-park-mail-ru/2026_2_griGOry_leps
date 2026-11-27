package delivery

import (
	"errors"
	"log/slog"
	"net/http"

	authclient "github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/client/auth"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

// writeUsersError отвечает на ошибки справочника пользователей (auth) и возвращает true, если ответ записан.
func writeUsersError(w http.ResponseWriter, r *http.Request, err error) bool {
	var status int
	var message string
	switch {
	case errors.Is(err, authclient.ErrUnavailable):
		status, message = http.StatusServiceUnavailable, "users service unavailable"
	case errors.Is(err, authclient.ErrTimeout):
		status, message = http.StatusGatewayTimeout, "users service timeout"
	case errors.Is(err, authclient.ErrBadResponse), errors.Is(err, authclient.ErrInternal), errors.Is(err, domain.ErrUserMissing):
		status, message = http.StatusBadGateway, "invalid auth service response"
	default:
		return false
	}
	slog.ErrorContext(r.Context(), "users directory", "err", err)
	writeError(r.Context(), w, status, message)
	return true
}
