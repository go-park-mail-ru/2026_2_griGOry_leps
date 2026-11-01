package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestOrderResponse(t *testing.T) {
	h := NewOrderHandler(nil)
	until := time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC)
	o := domain.Order{
		ID: 7, Status: domain.OrderAwaitingPayment, PaymentMethod: domain.PaymentCard, PaymentStatus: domain.PaymentPending,
		PayUntil: &until, ItemsTotal: 1500050, DeliveryTotal: 39000,
	}

	resp := h.newOrderResponse(o)
	require.Equal(t, int32(10007), resp.Number)
	require.Equal(t, "15390.50", resp.Totals.Total)
	require.Equal(t, &until, resp.PayUntil)

	o.Status, o.PaymentStatus = domain.OrderPlaced, domain.PaymentOnDelivery
	resp = h.newOrderResponse(o)
	require.Nil(t, resp.Payment.ConfirmationURL)
	require.Nil(t, resp.PayUntil)
}

func TestOrderErrors(t *testing.T) {
	tests := []struct {
		err  error
		code int
		body string
	}{
		{domain.ErrDeliveryTimeSlot, http.StatusBadRequest, `{"error":"delivery time slot is required","field":"delivery.time_slot"}`},
		{domain.ErrInvalidPhone, http.StatusBadRequest, `{"error":"invalid phone number","field":"recipient.phone"}`},
		{domain.ErrInvalidIdempotency, http.StatusBadRequest, `{"error":"invalid Idempotency-Key header"}`},
		{domain.ErrOrderNotFound, http.StatusNotFound, `{"error":"order not found"}`},
		{&domain.ItemsUnavailableError{ListingIDs: []int32{5}}, http.StatusConflict, `{"error":"cart items are unavailable","listing_ids":[5]}`},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		writeOrderError(rec, httptest.NewRequest(http.MethodPost, "/api/orders", nil), "test", tt.err)
		require.Equal(t, tt.code, rec.Code, tt.err.Error())
		require.JSONEq(t, tt.body, rec.Body.String())
	}
}
