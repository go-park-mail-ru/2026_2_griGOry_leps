package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type deleteOnlineconfService struct {
	onlineconfService
	err   error
	calls []int
	actor int32
}

func (s *deleteOnlineconfService) Delete(_ context.Context, actor int32, id int) error {
	s.calls = append(s.calls, id)
	s.actor = actor
	return s.err
}

func TestOnlineConfRouter_DeleteEntry(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		err    error
		status int
		calls  []int
	}{
		{"удаляет запись", "/entries/5", nil, http.StatusNoContent, []int{5}},
		{"запись не найдена", "/entries/5", domain.ErrOnlineconfMissing, http.StatusNotFound, []int{5}},
		{"сбой хранилища", "/entries/5", context.DeadlineExceeded, http.StatusInternalServerError, []int{5}},
		{"нулевой id", "/entries/0", nil, http.StatusBadRequest, nil},
		{"нечисловой id", "/entries/abc", nil, http.StatusBadRequest, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captureLogs(t)
			ctrl := gomock.NewController(t)
			checker := NewMockSessionChecker(ctrl)
			access := NewMockOnlineConfAccess(ctrl)
			checker.EXPECT().Me(gomock.Any(), "s").Return(domain.User{ID: 7}, nil)
			access.EXPECT().CheckAdmin(gomock.Any(), int32(7)).Return(nil)
			csrf := newCSRFHandler()
			service := &deleteOnlineconfService{err: tc.err}
			handler := newOnlineConfAPIRouter(NewSessionMiddleware(checker), csrf, NewOnlineConfAccessHandler(access), service)

			cookie := &http.Cookie{Name: sessionCookieName, Value: "s"}
			token, _ := fetchCSRFToken(t, csrf, cookie)
			req := httptest.NewRequest(http.MethodDelete, onlineConfAdminPrefix+tc.path, nil)
			req.AddCookie(cookie)
			req.Header.Set(csrfHeader, token)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			require.Equal(t, tc.status, rec.Code, rec.Body.String())
			require.Equal(t, tc.calls, service.calls)
			if tc.status == http.StatusNoContent {
				require.Empty(t, rec.Body.String())
				require.Equal(t, int32(7), service.actor)
			}
		})
	}
}

func TestOnlineConfRouter_DeleteEntryRequiresCSRF(t *testing.T) {
	r := newOnlineConfTestRouter(t)
	req := httptest.NewRequest(http.MethodDelete, onlineConfAdminPrefix+"/entries/1", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	r.handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"error":"invalid csrf token"}`, rec.Body.String())
}
