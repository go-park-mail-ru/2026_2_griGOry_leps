package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func testAddress(id int32) domain.Address {
	return domain.Address{ID: id, UserID: 7, City: "Москва", Street: "ул. Тверская", House: "д. 12", Apartment: "5",
		Comment: "Домофон 34", IsDefault: true}
}

func addressHTTPRequest(method, body, id string) *http.Request {
	req := httptest.NewRequest(method, "/api/me/addresses", strings.NewReader(body))
	if id != "" {
		req = mux.SetURLVars(req, map[string]string{"address_id": id})
	}
	return withAuth(req)
}

func TestAddress_List(t *testing.T) {
	mock := NewMockAddressUsecase(gomock.NewController(t))
	mock.EXPECT().List(gomock.Any(), int32(7)).Return([]domain.Address{testAddress(1)}, nil)
	rec := httptest.NewRecorder()

	NewAddressHandler(mock).List(rec, addressHTTPRequest(http.MethodGet, "", ""))

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `[{
		"id": 1, "city": "Москва", "street": "ул. Тверская", "house": "д. 12", "apartment": "5",
		"comment": "Домофон 34", "is_default": true
	}]`, rec.Body.String())
}

func TestAddress_Create(t *testing.T) {
	mock := NewMockAddressUsecase(gomock.NewController(t))
	city, street, house, apartment, comment, isDefault := "Москва", "Тверская", "д. 12", "5", "", true
	mock.EXPECT().Create(gomock.Any(), int32(7), domain.AddressInput{City: &city, Street: &street, House: &house, Apartment: &apartment, Comment: &comment, IsDefault: &isDefault}).Return(testAddress(2), nil)
	rec := httptest.NewRecorder()

	NewAddressHandler(mock).Create(rec, addressHTTPRequest(http.MethodPost,
		`{"city":"Москва","street":"Тверская","house":"д. 12","apartment":"5","comment":"","is_default":true}`, ""))

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, float64(2), decodeBody(t, rec)["id"])
}

func TestAddress_Create_FieldErrors(t *testing.T) {
	tests := []struct {
		err   error
		field string
	}{
		{domain.ErrMissingCity, "city"},
		{domain.ErrCityTooLong, "city"},
		{domain.ErrMissingStreet, "street"},
		{domain.ErrStreetTooLong, "street"},
		{domain.ErrMissingHouse, "house"},
		{domain.ErrHouseTooLong, "house"},
		{domain.ErrApartmentTooLong, "apartment"},
		{domain.ErrAddressCommentTooLong, "comment"},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		mock := NewMockAddressUsecase(gomock.NewController(t))
		mock.EXPECT().Create(gomock.Any(), int32(7), domain.AddressInput{}).Return(domain.Address{}, tt.err)
		NewAddressHandler(mock).Create(rec, addressHTTPRequest(http.MethodPost, `{}`, ""))

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.Equal(t, map[string]any{"error": tt.err.Error(), "field": tt.field}, decodeBody(t, rec))
	}

	rec := httptest.NewRecorder()
	NewAddressHandler(NewMockAddressUsecase(gomock.NewController(t))).Create(rec, addressHTTPRequest(http.MethodPost, `[`, ""))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "invalid request body", decodeBody(t, rec)["error"])
}

func TestAddress_Update(t *testing.T) {
	mock := NewMockAddressUsecase(gomock.NewController(t))
	isDefault := false
	mock.EXPECT().Update(gomock.Any(), int32(7), int32(4), domain.AddressInput{IsDefault: &isDefault}).Return(testAddress(4), nil)
	rec := httptest.NewRecorder()

	NewAddressHandler(mock).Update(rec, addressHTTPRequest(http.MethodPatch, `{"is_default":false}`, "4"))

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAddress_Update_NotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	mock := NewMockAddressUsecase(gomock.NewController(t))
	mock.EXPECT().Update(gomock.Any(), int32(7), int32(4), domain.AddressInput{}).Return(domain.Address{}, domain.ErrAddressNotFound)
	NewAddressHandler(mock).Update(rec, addressHTTPRequest(http.MethodPatch, `{}`, "4"))
	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "address not found", decodeBody(t, rec)["error"])
}

func TestAddress_Delete(t *testing.T) {
	mock := NewMockAddressUsecase(gomock.NewController(t))
	mock.EXPECT().Delete(gomock.Any(), int32(7), int32(4)).Return(nil)
	rec := httptest.NewRecorder()

	NewAddressHandler(mock).Delete(rec, addressHTTPRequest(http.MethodDelete, "", "4"))

	require.Equal(t, http.StatusNoContent, rec.Code)
}
