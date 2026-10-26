package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type addressStub struct {
	err     error
	userID  int32
	id      int32
	in      domain.AddressInput
	deleted bool
}

func testAddress(id int32) domain.Address {
	return domain.Address{ID: id, UserID: 7, Street: "Москва, ул. Тверская", House: "д. 12", Comment: "Домофон 34", IsDefault: true}
}

func (s *addressStub) List(_ context.Context, userID int32) ([]domain.Address, error) {
	s.userID = userID
	return []domain.Address{testAddress(1)}, s.err
}

func (s *addressStub) Create(_ context.Context, userID int32, in domain.AddressInput) (domain.Address, error) {
	s.userID, s.in = userID, in
	return testAddress(2), s.err
}

func (s *addressStub) Update(_ context.Context, userID, id int32, in domain.AddressInput) (domain.Address, error) {
	s.userID, s.id, s.in = userID, id, in
	return testAddress(id), s.err
}

func (s *addressStub) Delete(_ context.Context, userID, id int32) error {
	s.userID, s.id, s.deleted = userID, id, true
	return s.err
}

func addressHTTPRequest(method, body, id string) *http.Request {
	req := httptest.NewRequest(method, "/api/me/addresses", strings.NewReader(body))
	if id != "" {
		req = mux.SetURLVars(req, map[string]string{"address_id": id})
	}
	return withAuth(req)
}

func TestAddress_List(t *testing.T) {
	stub := &addressStub{}
	rec := httptest.NewRecorder()

	NewAddressHandler(stub).List(rec, addressHTTPRequest(http.MethodGet, "", ""))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(7), stub.userID)
	require.JSONEq(t, `[{
		"id": 1, "street": "Москва, ул. Тверская", "house": "д. 12", "comment": "Домофон 34", "is_default": true
	}]`, rec.Body.String())
}

func TestAddress_Create(t *testing.T) {
	stub := &addressStub{}
	rec := httptest.NewRecorder()

	NewAddressHandler(stub).Create(rec, addressHTTPRequest(http.MethodPost,
		`{"street":"Тверская","house":"д. 12","comment":"","is_default":true}`, ""))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, "Тверская", *stub.in.Street)
	require.Equal(t, "д. 12", *stub.in.House)
	require.Equal(t, "", *stub.in.Comment)
	require.True(t, *stub.in.IsDefault)
	require.Equal(t, float64(2), decodeBody(t, rec)["id"])
}

func TestAddress_Create_FieldErrors(t *testing.T) {
	tests := []struct {
		err   error
		field string
	}{
		{domain.ErrMissingStreet, "street"},
		{domain.ErrStreetTooLong, "street"},
		{domain.ErrMissingHouse, "house"},
		{domain.ErrHouseTooLong, "house"},
		{domain.ErrAddressCommentTooLong, "comment"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		NewAddressHandler(&addressStub{err: tt.err}).Create(rec, addressHTTPRequest(http.MethodPost, `{}`, ""))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, map[string]any{"error": tt.err.Error(), "field": tt.field}, decodeBody(t, rec))
	}

	rec := httptest.NewRecorder()
	NewAddressHandler(&addressStub{}).Create(rec, addressHTTPRequest(http.MethodPost, `[`, ""))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid request body", decodeBody(t, rec)["error"])
}

func TestAddress_Update(t *testing.T) {
	stub := &addressStub{}
	rec := httptest.NewRecorder()

	NewAddressHandler(stub).Update(rec, addressHTTPRequest(http.MethodPatch, `{"is_default":false}`, "4"))

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, int32(4), stub.id)
	require.Nil(t, stub.in.Street)
	require.False(t, *stub.in.IsDefault)
}

func TestAddress_Update_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	NewAddressHandler(&addressStub{err: domain.ErrAddressNotFound}).Update(rec, addressHTTPRequest(http.MethodPatch, `{}`, "4"))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "address not found", decodeBody(t, rec)["error"])
}

func TestAddress_Delete(t *testing.T) {
	stub := &addressStub{}
	rec := httptest.NewRecorder()

	NewAddressHandler(stub).Delete(rec, addressHTTPRequest(http.MethodDelete, "", "4"))

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.True(t, stub.deleted)
	require.Equal(t, int32(4), stub.id)
}
