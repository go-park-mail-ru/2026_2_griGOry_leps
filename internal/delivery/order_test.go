package delivery

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func TestFormatRubles(t *testing.T) {
	require.Equal(t, "0", formatRubles(0))
	require.Equal(t, "990", formatRubles(99000))
	require.Equal(t, "8\u00a0400", formatRubles(840000))
	require.Equal(t, "1\u00a0234\u00a0567,50", formatRubles(123456750))
}

func TestOrderResponse_ConfirmationURLOnlyWhilePending(t *testing.T) {
	h := NewOrderHandler(nil, "http://api.test", "http://front.test")
	token := "abc"
	until := time.Date(2026, 10, 26, 13, 0, 0, 0, time.UTC)
	o := domain.Order{
		ID: 7, Status: domain.OrderAwaitingPayment, PaymentMethod: domain.PaymentCard, PaymentStatus: domain.PaymentPending,
		PaymentToken: &token, PayUntil: &until, ItemsTotal: 1500050, DeliveryTotal: 39000,
	}

	resp := h.newOrderResponse(o)
	require.Equal(t, int32(10007), resp.Number)
	require.Equal(t, "http://api.test/api/test-payments/abc", *resp.Payment.ConfirmationURL)
	require.Equal(t, "15390.50", resp.Totals.Total)
	require.Equal(t, &until, resp.PayUntil)

	o.PaymentStatus = domain.PaymentFailed
	require.Nil(t, h.newOrderResponse(o).Payment.ConfirmationURL)

	o.Status, o.PaymentStatus = domain.OrderPlaced, domain.PaymentPaid
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
		{domain.ErrOrderNotPayable, http.StatusConflict, `{"error":"order is not payable"}`},
	}
	for _, tt := range tests {
		rec := httptest.NewRecorder()
		writeOrderError(rec, httptest.NewRequest(http.MethodPost, "/api/orders", nil), "test", tt.err)
		require.Equal(t, tt.code, rec.Code, tt.err.Error())
		require.JSONEq(t, tt.body, rec.Body.String())
	}
}

func servePayment(t *testing.T, mock *MockOrderUsecase, path string) *httptest.ResponseRecorder {
	t.Helper()
	h := NewOrderHandler(mock, "http://api.test", "http://front.test")
	r := mux.NewRouter()
	r.HandleFunc(testPaymentPath+"{token}", h.TestPaymentPage)
	r.HandleFunc(testPaymentPath+"{token}/success", h.TestPaymentResult(true))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, testPaymentPath+path, nil))
	return rec
}

func TestTestPayment_PendingOrderShowsBank(t *testing.T) {
	mock := NewMockOrderUsecase(gomock.NewController(t))
	mock.EXPECT().PaymentPage(gomock.Any(), "abc").Return(domain.Order{
		ID: 7, Status: domain.OrderAwaitingPayment, PaymentStatus: domain.PaymentPending, PaymentMethod: domain.PaymentSBP, ItemsTotal: 99000,
	}, nil)
	rec := servePayment(t, mock, "abc")

	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	require.Contains(t, rec.Body.String(), "Заказ № 10007 · СБП")
	require.Contains(t, rec.Body.String(), testPaymentPath+"abc/success")
}

func TestTestPayment_HandledOrderRedirectsToOrderPage(t *testing.T) {
	for _, o := range []domain.Order{
		{ID: 7, Status: domain.OrderAwaitingPayment, PaymentStatus: domain.PaymentProcessing},
		{ID: 7, Status: domain.OrderAwaitingPayment, PaymentStatus: domain.PaymentFailed},
		{ID: 7, Status: domain.OrderPlaced, PaymentStatus: domain.PaymentPaid},
	} {
		mock := NewMockOrderUsecase(gomock.NewController(t))
		mock.EXPECT().PaymentPage(gomock.Any(), "abc").Return(o, nil)
		rec := servePayment(t, mock, "abc")

		require.Equal(t, http.StatusSeeOther, rec.Code, o.PaymentStatus)
		require.Equal(t, "http://front.test/orders/7", rec.Header().Get("Location"))
	}
}

func TestTestPayment_StaleLinkShowsPage(t *testing.T) {
	for _, path := range []string{"abc", "abc/success"} {
		mock := NewMockOrderUsecase(gomock.NewController(t))
		if path == "abc" {
			mock.EXPECT().PaymentPage(gomock.Any(), "abc").Return(domain.Order{}, domain.ErrOrderNotFound)
		} else {
			mock.EXPECT().TestPayment(gomock.Any(), "abc", true).Return(int32(0), domain.ErrOrderNotFound)
		}
		rec := servePayment(t, mock, path)

		require.Equal(t, http.StatusNotFound, rec.Code, path)
		require.Contains(t, rec.Header().Get("Content-Type"), "text/html", path)
		require.Contains(t, rec.Body.String(), "Ссылка на оплату устарела", path)
		require.Contains(t, rec.Body.String(), `href="http://front.test"`, path)
	}
}
