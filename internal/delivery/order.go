package delivery

import (
	"net/http"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OrderHandler struct {
	orders OrderUsecase
}

func NewOrderHandler(orders OrderUsecase) *OrderHandler {
	return &OrderHandler{orders: orders}
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

func (h *OrderHandler) Quote(w http.ResponseWriter, r *http.Request) {
	var req listingIDsRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	q, err := h.orders.Quote(r.Context(), userIDFromContext(r.Context()), req.ListingIDs)
	if err != nil {
		writeCartError(w, r, "checkout quote", err)
		return
	}
	writeJSON(r.Context(), w, http.StatusOK, newQuoteResponse(q))
}
