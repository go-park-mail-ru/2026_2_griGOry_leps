package delivery

import (
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	testPaymentPath   = "/api/test-payments/"
	idempotencyHeader = "Idempotency-Key"
)

type OrderHandler struct {
	orders         OrderUsecase
	publicURL      string
	frontendOrigin string
}

func NewOrderHandler(orders OrderUsecase, publicURL, frontendOrigin string) *OrderHandler {
	return &OrderHandler{orders: orders, publicURL: publicURL, frontendOrigin: frontendOrigin}
}

type orderItemResponse struct {
	ListingID *int32  `json:"listing_id"`
	SellerID  int32   `json:"seller_id"`
	Title     string  `json:"title"`
	Price     string  `json:"price"`
	ImageURL  *string `json:"image_url"`
}

type deliveryOptionResponse struct {
	Method    string `json:"method"`
	Price     string `json:"price"`
	Available bool   `json:"available"`
	Hint      string `json:"hint"`
}

type quoteSellerResponse struct {
	SellerID        int32                    `json:"seller_id"`
	FirstName       string                   `json:"first_name"`
	ItemsCount      int                      `json:"items_count"`
	DeliveryMethods []deliveryOptionResponse `json:"delivery_methods"`
}

type quoteAddressResponse struct {
	ID        int32  `json:"id"`
	Label     string `json:"label"`
	Text      string `json:"text"`
	IsDefault bool   `json:"is_default"`
}

type recipientJSON struct {
	FirstName string `json:"first_name"`
	Phone     string `json:"phone"`
}

type paymentMethodResponse struct {
	Method string `json:"method"`
}

type deliverySlotsResponse struct {
	Dates []string `json:"dates"`
	Times []string `json:"times"`
}

type quoteResponse struct {
	Items          []orderItemResponse     `json:"items"`
	ItemsTotal     string                  `json:"items_total"`
	Sellers        []quoteSellerResponse   `json:"sellers"`
	Addresses      []quoteAddressResponse  `json:"addresses"`
	DeliverySlots  deliverySlotsResponse   `json:"delivery_slots"`
	Recipient      recipientJSON           `json:"recipient"`
	PaymentMethods []paymentMethodResponse `json:"payment_methods"`
}

type sellerDeliveryJSON struct {
	SellerID int32  `json:"seller_id"`
	Method   string `json:"method"`
}

type newAddressJSON struct {
	Text string `json:"text"`
}

type orderDeliveryRequest struct {
	BySeller  []sellerDeliveryJSON `json:"by_seller"`
	AddressID *int32               `json:"address_id"`
	Address   *newAddressJSON      `json:"address"`
	Date      *string              `json:"date"`
	TimeSlot  *string              `json:"time_slot"`
	Comment   string               `json:"comment"`
}

type createOrderRequest struct {
	ListingIDs    []int32              `json:"listing_ids"`
	Delivery      orderDeliveryRequest `json:"delivery"`
	Recipient     recipientJSON        `json:"recipient"`
	PaymentMethod string               `json:"payment_method"`
}

type payOrderRequest struct {
	Method string `json:"method"`
}

type orderTotalsResponse struct {
	Items    string `json:"items"`
	Delivery string `json:"delivery"`
	Total    string `json:"total"`
}

type orderDeliveryResponse struct {
	BySeller    []sellerDeliveryJSON `json:"by_seller"`
	AddressID   *int32               `json:"address_id"`
	Address     *newAddressJSON      `json:"address"`
	Date        *string              `json:"date"`
	TimeSlot    *string              `json:"time_slot"`
	Comment     string               `json:"comment"`
	AddressText *string              `json:"address_text"`
}

type orderPaymentResponse struct {
	Method          string  `json:"method"`
	Status          string  `json:"status"`
	ConfirmationURL *string `json:"confirmation_url"`
}

type orderResponse struct {
	ID        int32                 `json:"id"`
	Number    int32                 `json:"number"`
	Status    string                `json:"status"`
	PayUntil  *time.Time            `json:"pay_until"`
	Items     []orderItemResponse   `json:"items"`
	Totals    orderTotalsResponse   `json:"totals"`
	Delivery  orderDeliveryResponse `json:"delivery"`
	Payment   orderPaymentResponse  `json:"payment"`
	CreatedAt time.Time             `json:"created_at"`
}

func newOrderItems(items []domain.OrderItem) []orderItemResponse {
	resp := make([]orderItemResponse, 0, len(items))
	for _, it := range items {
		resp = append(resp, orderItemResponse{
			ListingID: it.ListingID, SellerID: it.SellerID, Title: it.Title, Price: formatPrice(it.Price), ImageURL: it.ImageURL,
		})
	}
	return resp
}

func newQuoteResponse(q domain.CheckoutQuote) quoteResponse {
	resp := quoteResponse{
		Items:          newOrderItems(q.Items),
		ItemsTotal:     formatPrice(q.ItemsTotal),
		Sellers:        make([]quoteSellerResponse, 0, len(q.Sellers)),
		Addresses:      make([]quoteAddressResponse, 0, len(q.Addresses)),
		DeliverySlots:  deliverySlotsResponse{Dates: q.Dates, Times: q.Times},
		Recipient:      recipientJSON{FirstName: q.Recipient.FirstName, Phone: q.Recipient.Phone},
		PaymentMethods: make([]paymentMethodResponse, 0, len(q.Payments)),
	}
	for _, s := range q.Sellers {
		seller := quoteSellerResponse{SellerID: s.SellerID, FirstName: s.FirstName, ItemsCount: s.ItemsCount}
		for _, o := range s.Options {
			seller.DeliveryMethods = append(seller.DeliveryMethods, deliveryOptionResponse{
				Method: o.Method, Price: formatPrice(o.Price), Available: o.Available, Hint: o.Hint,
			})
		}
		resp.Sellers = append(resp.Sellers, seller)
	}
	for _, a := range q.Addresses {
		resp.Addresses = append(resp.Addresses, quoteAddressResponse{ID: a.ID, Label: a.Label, Text: a.Text, IsDefault: a.IsDefault})
	}
	for _, m := range q.Payments {
		resp.PaymentMethods = append(resp.PaymentMethods, paymentMethodResponse{Method: m})
	}
	return resp
}

func (h *OrderHandler) paymentURL(token string) string {
	return h.publicURL + testPaymentPath + token
}

func (h *OrderHandler) newOrderResponse(o domain.Order) orderResponse {
	resp := orderResponse{
		ID:     o.ID,
		Number: o.Number(),
		Status: o.Status,
		Items:  newOrderItems(o.Items),
		Totals: orderTotalsResponse{
			Items:    formatPrice(o.ItemsTotal),
			Delivery: formatPrice(o.DeliveryTotal),
			Total:    formatPrice(o.ItemsTotal + o.DeliveryTotal),
		},
		Delivery: orderDeliveryResponse{
			BySeller:    make([]sellerDeliveryJSON, 0, len(o.BySeller)),
			AddressID:   o.AddressID,
			Date:        o.Date,
			TimeSlot:    o.TimeSlot,
			Comment:     o.Comment,
			AddressText: o.AddressText,
		},
		Payment:   orderPaymentResponse{Method: o.PaymentMethod, Status: o.PaymentStatus},
		CreatedAt: o.CreatedAt,
	}
	for _, s := range o.BySeller {
		resp.Delivery.BySeller = append(resp.Delivery.BySeller, sellerDeliveryJSON{SellerID: s.SellerID, Method: s.Method})
	}
	if o.AddressID == nil && o.AddressText != nil {
		resp.Delivery.Address = &newAddressJSON{Text: *o.AddressText}
	}
	if o.Status == domain.OrderAwaitingPayment {
		resp.PayUntil = o.PayUntil
		if o.PaymentStatus == domain.PaymentPending && o.PaymentToken != nil {
			url := h.paymentURL(*o.PaymentToken)
			resp.Payment.ConfirmationURL = &url
		}
	}
	return resp
}

func (h *OrderHandler) Quote(w http.ResponseWriter, r *http.Request) {
	var req listingIDsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	q, err := h.orders.Quote(r.Context(), userIDFromContext(r.Context()), req.ListingIDs)
	if err != nil {
		writeOrderError(w, r, "checkout quote", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newQuoteResponse(q))
}

func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	in := domain.OrderInput{
		ListingIDs:     req.ListingIDs,
		BySeller:       make([]domain.SellerDelivery, 0, len(req.Delivery.BySeller)),
		AddressID:      req.Delivery.AddressID,
		Date:           req.Delivery.Date,
		TimeSlot:       req.Delivery.TimeSlot,
		Comment:        req.Delivery.Comment,
		Recipient:      domain.Recipient{FirstName: req.Recipient.FirstName, Phone: req.Recipient.Phone},
		PaymentMethod:  req.PaymentMethod,
		IdempotencyKey: r.Header.Get(idempotencyHeader),
	}
	for _, s := range req.Delivery.BySeller {
		in.BySeller = append(in.BySeller, domain.SellerDelivery{SellerID: s.SellerID, Method: s.Method})
	}
	if req.Delivery.Address != nil {
		in.AddressText = &req.Delivery.Address.Text
	}

	order, err := h.orders.Create(r.Context(), userIDFromContext(r.Context()), in)
	if err != nil {
		writeOrderError(w, r, "create order", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusCreated, h.newOrderResponse(order))
}

func orderID(r *http.Request) (int32, bool) {
	id, err := strconv.ParseInt(mux.Vars(r)["order_id"], 10, 32)
	return int32(id), err == nil && id > 0
}

func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}
	order, err := h.orders.Get(r.Context(), userIDFromContext(r.Context()), id)
	if err != nil {
		writeOrderError(w, r, "get order", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, h.newOrderResponse(order))
}

func (h *OrderHandler) Pay(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(r)
	if !ok {
		writeError(r.Context(), w, http.StatusNotFound, domain.ErrOrderNotFound.Error())
		return
	}
	var req payOrderRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	order, err := h.orders.Pay(r.Context(), userIDFromContext(r.Context()), id, req.Method)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidPaymentMethod) {
			writeFieldError(r.Context(), w, http.StatusBadRequest, "method", err.Error())
			return
		}
		writeOrderError(w, r, "retry payment", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, h.newOrderResponse(order))
}

var testPaymentPages = template.Must(template.New("pages").Parse(`{{define "head"}}<!doctype html>
<html lang="ru">
<head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.}}</title>
<style>
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;background:#f4f4f5;font-family:system-ui,sans-serif;color:#141417}
main{width:min(380px,calc(100% - 32px));padding:28px;background:#fff;border-radius:16px;box-shadow:0 4px 24px rgba(0,0,0,.08)}
h1{margin:0 0 6px;font-size:20px}p{margin:0 0 20px;color:#525259}strong{display:block;margin-bottom:24px;font-size:28px}
a{display:block;margin-top:10px;padding:12px;border-radius:10px;text-align:center;text-decoration:none;font-weight:600}
.ok{background:#0866ff;color:#fff}.fail{background:#f4f4f5;color:#141417}
</style></head>{{end}}
{{define "pay"}}{{template "head" "Тестовая оплата"}}
<body><main>
<h1>Тестовый банк</h1>
<p>Заказ № {{.Number}} · {{.Method}}. Деньги не списываются, это заглушка вместо платёжного провайдера.</p>
<strong>{{.Total}} ₽</strong>
<a id="ok" class="ok" href="{{.OK}}">Оплатить</a>
<a id="fail" class="fail" href="{{.Fail}}">Отклонить</a>
</main></body></html>{{end}}
{{define "stale"}}{{template "head" "Ссылка на оплату устарела"}}
<body><main>
<h1>Ссылка на оплату устарела</h1>
<p>Заказ уже оплачен или отменён, либо оплату начали заново по новой ссылке. Деньги по этой ссылке не спишутся.</p>
<a class="ok" href="{{.}}">Вернуться в GoGET</a>
</main></body></html>{{end}}`))

func (h *OrderHandler) TestPaymentPage(w http.ResponseWriter, r *http.Request) {
	token := mux.Vars(r)["token"]
	order, err := h.orders.PaymentPage(r.Context(), token)
	if errors.Is(err, domain.ErrOrderNotFound) {
		h.renderPaymentPage(w, r, http.StatusNotFound, "stale", h.frontendOrigin)
		return
	}
	if err != nil {
		writeOrderError(w, r, "test payment page", err)
		return
	}
	if order.Status != domain.OrderAwaitingPayment || order.PaymentStatus != domain.PaymentPending {
		http.Redirect(w, r, h.orderPageURL(order.ID), http.StatusSeeOther)
		return
	}

	method := "Картой"
	if order.PaymentMethod == domain.PaymentSBP {
		method = "СБП"
	}
	h.renderPaymentPage(w, r, http.StatusOK, "pay", map[string]any{
		"Number": order.Number(),
		"Method": method,
		"Total":  formatRubles(order.ItemsTotal + order.DeliveryTotal),
		"OK":     testPaymentPath + token + "/success",
		"Fail":   testPaymentPath + token + "/fail",
	})
}

func (h *OrderHandler) TestPaymentResult(success bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := h.orders.TestPayment(r.Context(), mux.Vars(r)["token"], success)
		if errors.Is(err, domain.ErrOrderNotFound) {
			h.renderPaymentPage(w, r, http.StatusNotFound, "stale", h.frontendOrigin)
			return
		}
		if err != nil {
			writeOrderError(w, r, "test payment", err)
			return
		}
		http.Redirect(w, r, h.orderPageURL(id), http.StatusSeeOther)
	}
}

func (h *OrderHandler) orderPageURL(id int32) string {
	return fmt.Sprintf("%s/orders/%d", h.frontendOrigin, id)
}

func (h *OrderHandler) renderPaymentPage(w http.ResponseWriter, r *http.Request, status int, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := testPaymentPages.ExecuteTemplate(w, page, data); err != nil {
		slog.ErrorContext(r.Context(), "render test payment page", "page", page, "err", err)
	}
}

func formatRubles(kopecks int64) string {
	digits := strconv.FormatInt(kopecks/100, 10)
	var b strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteRune('\u00a0')
		}
		b.WriteRune(r)
	}
	if rest := kopecks % 100; rest != 0 {
		fmt.Fprintf(&b, ",%02d", rest)
	}
	return b.String()
}

func orderErrorField(err error) string {
	switch {
	case errors.Is(err, domain.ErrDeliveryNotAvailable):
		return "delivery.by_seller"
	case errors.Is(err, domain.ErrDeliveryAddress):
		return "delivery.address"
	case errors.Is(err, domain.ErrDeliveryDate):
		return "delivery.date"
	case errors.Is(err, domain.ErrDeliveryTimeSlot):
		return "delivery.time_slot"
	case errors.Is(err, domain.ErrOrderCommentTooLong):
		return "delivery.comment"
	case errors.Is(err, domain.ErrMissingFirstName), errors.Is(err, domain.ErrInvalidFirstName):
		return "recipient.first_name"
	case errors.Is(err, domain.ErrInvalidPhone):
		return "recipient.phone"
	case errors.Is(err, domain.ErrInvalidPaymentMethod):
		return "payment_method"
	}
	return ""
}

func writeOrderError(w http.ResponseWriter, r *http.Request, op string, err error) {
	ctx := r.Context()
	if field := orderErrorField(err); field != "" {
		writeFieldError(ctx, w, http.StatusBadRequest, field, err.Error())
		return
	}
	switch {
	case errors.Is(err, domain.ErrInvalidIdempotency):
		writeError(ctx, w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrOrderNotFound):
		writeError(ctx, w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrOrderNotPayable):
		writeError(ctx, w, http.StatusConflict, err.Error())
	default:
		writeCartError(w, r, op, err)
	}
}
