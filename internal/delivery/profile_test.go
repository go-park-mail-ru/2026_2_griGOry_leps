package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type profileStub struct {
	err            error
	update         func(domain.ProfileUpdate)
	changePassword func(sessionID, current, next string)
	upload         func(data []byte)
}

func (s profileStub) Profile(context.Context, int32) (domain.Profile, error) {
	return testProfile(), s.err
}

func (s profileStub) UpdateProfile(_ context.Context, _ int32, upd domain.ProfileUpdate) (domain.Profile, error) {
	if s.update != nil {
		s.update(upd)
	}
	return testProfile(), s.err
}

func (s profileStub) ChangePassword(_ context.Context, _ int32, sessionID, current, next string) error {
	if s.changePassword != nil {
		s.changePassword(sessionID, current, next)
	}
	return s.err
}

func (s profileStub) UploadAvatar(_ context.Context, _ int32, data []byte) (string, error) {
	if s.upload != nil {
		s.upload(data)
	}
	return "abc.png", s.err
}

func testProfile() domain.Profile {
	return domain.Profile{
		User: domain.User{
			ID:        7,
			FirstName: "Анна",
			Email:     "anna@goget.pw",
			Phone:     "+79990000001",
			CreatedAt: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
		},
		City:         &domain.City{ID: 1, Name: "Москва"},
		About:        "Продаю вещи",
		Rating:       4.5,
		ReviewsCount: 2,
	}
}

func withAuth(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "current-session"})
	return req.WithContext(context.WithValue(req.Context(), userIDKey{}, int32(7)))
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestProfile_Get(t *testing.T) {
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{}).Get(rec, withAuth(httptest.NewRequest(http.MethodGet, "/api/me", nil)))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{
		"id": 7,
		"email": "anna@goget.pw",
		"first_name": "Анна",
		"phone": "+79990000001",
		"phone_verified": false,
		"city": {"id": 1, "name": "Москва"},
		"about": "Продаю вещи",
		"avatar_url": null,
		"rating": 4.5,
		"reviews_count": 2,
		"created_at": "2026-10-01T12:00:00Z"
	}`, rec.Body.String())
}

func TestProfile_Get_AvatarURL(t *testing.T) {
	p := testProfile()
	p.Avatar = "abc.jpg"
	p.City = nil

	resp := newProfileResponse(p)

	require.NotNil(t, resp.AvatarURL)
	require.Equal(t, "/uploads/avatars/abc.jpg", *resp.AvatarURL)
	require.Nil(t, resp.City)
}

func TestProfile_Update_PassesOnlySentFields(t *testing.T) {
	var got domain.ProfileUpdate
	h := NewProfileHandler(profileStub{update: func(upd domain.ProfileUpdate) { got = upd }})
	req := withAuth(httptest.NewRequest(http.MethodPatch, "/api/me", strings.NewReader(`{"about":"","city_id":3,"role":"admin"}`)))
	rec := httptest.NewRecorder()

	h.Update(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.NotNil(t, got.About)
	require.Empty(t, *got.About)
	require.EqualValues(t, 3, *got.CityID)
	require.Nil(t, got.FirstName)
	require.Nil(t, got.Phone)
	require.Nil(t, got.Email)
}

func TestProfile_Update_BadJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{}).Update(rec, withAuth(httptest.NewRequest(http.MethodPatch, "/api/me", strings.NewReader("{"))))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestProfile_Update_Errors(t *testing.T) {
	tests := []struct {
		err    error
		status int
		field  string
	}{
		{domain.ErrInvalidFirstName, http.StatusBadRequest, "first_name"},
		{domain.ErrInvalidPhone, http.StatusBadRequest, "phone"},
		{domain.ErrInvalidEmail, http.StatusBadRequest, "email"},
		{domain.ErrInvalidCity, http.StatusBadRequest, "city_id"},
		{domain.ErrAboutTooLong, http.StatusBadRequest, "about"},
		{domain.ErrEmailTaken, http.StatusConflict, "email"},
		{domain.ErrPhoneTaken, http.StatusConflict, "phone"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewProfileHandler(profileStub{err: tt.err}).Update(rec, withAuth(httptest.NewRequest(http.MethodPatch, "/api/me", strings.NewReader(`{}`))))

			require.Equal(t, tt.status, rec.Code)
			require.Equal(t, tt.field, decodeBody(t, rec)["field"])
		})
	}
}

func TestProfile_Update_InternalError(t *testing.T) {
	captureLogs(t)
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{err: errors.New("db down")}).Update(rec, withAuth(httptest.NewRequest(http.MethodPatch, "/api/me", strings.NewReader(`{}`))))

	require.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestProfile_ChangePassword_PassesCurrentSession(t *testing.T) {
	var gotSession, gotCurrent, gotNext string
	h := NewProfileHandler(profileStub{changePassword: func(sessionID, current, next string) {
		gotSession, gotCurrent, gotNext = sessionID, current, next
	}})
	body := `{"current_password":"Old12345","new_password":"New12345"}`
	rec := httptest.NewRecorder()

	h.ChangePassword(rec, withAuth(httptest.NewRequest(http.MethodPut, "/api/me/password", strings.NewReader(body))))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
	require.Equal(t, "current-session", gotSession)
	require.Equal(t, "Old12345", gotCurrent)
	require.Equal(t, "New12345", gotNext)
}

func TestProfile_ChangePassword_Errors(t *testing.T) {
	tests := []struct {
		err   error
		field string
	}{
		{domain.ErrWrongPassword, "current_password"},
		{domain.ErrWeakPassword, "new_password"},
		{domain.ErrPasswordTooLong, "new_password"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewProfileHandler(profileStub{err: tt.err}).ChangePassword(rec, withAuth(httptest.NewRequest(http.MethodPut, "/api/me/password", strings.NewReader(`{}`))))

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Equal(t, tt.field, decodeBody(t, rec)["field"])
		})
	}
}

func multipartRequest(t *testing.T, field string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(field, "avatar.png")
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/me/avatar", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return withAuth(req)
}

func TestProfile_UploadAvatar(t *testing.T) {
	var got []byte
	h := NewProfileHandler(profileStub{upload: func(data []byte) { got = data }})
	rec := httptest.NewRecorder()

	h.UploadAvatar(rec, multipartRequest(t, "file", []byte("png-bytes")))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"avatar_url":"/uploads/avatars/abc.png"}`, rec.Body.String())
	require.Equal(t, []byte("png-bytes"), got)
}

func TestProfile_UploadAvatar_NoFile(t *testing.T) {
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{}).UploadAvatar(rec, multipartRequest(t, "photo", []byte("x")))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"error":"file is required","field":"file"}`, rec.Body.String())
}

func TestProfile_UploadAvatar_NotMultipart(t *testing.T) {
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{}).UploadAvatar(rec, withAuth(httptest.NewRequest(http.MethodPost, "/api/me/avatar", strings.NewReader("{}"))))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestProfile_UploadAvatar_TooLarge(t *testing.T) {
	rec := httptest.NewRecorder()

	NewProfileHandler(profileStub{}).UploadAvatar(rec, multipartRequest(t, "file", make([]byte, maxAvatarBytes+1)))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

func TestProfile_UploadAvatar_Errors(t *testing.T) {
	tests := []struct {
		err    error
		status int
	}{
		{domain.ErrUnsupportedImage, http.StatusUnsupportedMediaType},
		{domain.ErrInvalidImage, http.StatusBadRequest},
		{domain.ErrImageTooLarge, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			rec := httptest.NewRecorder()

			NewProfileHandler(profileStub{err: tt.err}).UploadAvatar(rec, multipartRequest(t, "file", []byte("x")))

			require.Equal(t, tt.status, rec.Code)
		})
	}
}
