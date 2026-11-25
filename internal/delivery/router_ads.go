package delivery

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/ratelimit"
)

func NewAdsRouter(frontendOrigin, uploadDir string, session *SessionMiddleware, adHandler *AdHandler, listingHandler *ListingHandler, csrfHandler *CSRFHandler, cartHandler *CartHandler, addressHandler *AddressHandler, cityHandler *CityHandler, sellerHandler *SellerHandler, orderHandler *OrderHandler, favoritesHandler *FavoritesHandler) http.Handler {
	r := mux.NewRouter()
	r.Use(recoverMiddleware, csrfHandler.Middleware)
	r.NotFoundHandler = http.HandlerFunc(notFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(methodNotAllowed)

	r.HandleFunc("/api/health", func(w http.ResponseWriter, req *http.Request) {
		writeJSON(req.Context(), w, http.StatusOK, map[string]string{"status": "ok", "service": "ads"})
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/csrf", csrfHandler.Token).Methods(http.MethodGet)

	auth := func(h http.HandlerFunc) http.Handler { return session.RequireAuth(h) }
	verified := func(h http.HandlerFunc) http.Handler { return session.RequireAuth(requireVerifiedEmail(h)) }
	reportLimit := ratelimit.New(5, 10*time.Minute)
	const favoritePath = "/api/favorites/{listing_id:[0-9]+}"
	r.Handle("/api/favorites", auth(favoritesHandler.List)).Methods(http.MethodGet)
	r.Handle(favoritePath, auth(favoritesHandler.Add)).Methods(http.MethodPut)
	r.Handle(favoritePath, auth(favoritesHandler.Remove)).Methods(http.MethodDelete)
	r.Handle("/api/me/listings", auth(adHandler.ListMine)).Methods(http.MethodGet)

	const addressPath = "/api/me/addresses/{address_id:[0-9]+}"
	r.Handle("/api/me/addresses", auth(addressHandler.List)).Methods(http.MethodGet)
	r.Handle("/api/me/addresses", verified(addressHandler.Create)).Methods(http.MethodPost)
	r.Handle(addressPath, verified(addressHandler.Update)).Methods(http.MethodPatch)
	r.Handle(addressPath, verified(addressHandler.Delete)).Methods(http.MethodDelete)

	r.HandleFunc("/api/cities", cityHandler.Search).Methods(http.MethodGet)

	const sellerPath = "/api/sellers/{seller_id:[0-9]+}"
	r.HandleFunc(sellerPath, sellerHandler.Get).Methods(http.MethodGet)
	r.HandleFunc(sellerPath+"/listings", sellerHandler.Listings).Methods(http.MethodGet)
	r.HandleFunc(sellerPath+"/reviews", sellerHandler.Reviews).Methods(http.MethodGet)
	r.Handle(sellerPath+"/reports", verified(rateLimit(reportLimit, sellerHandler.Report))).Methods(http.MethodPost)

	r.HandleFunc("/api/categories", listingHandler.Categories).Methods(http.MethodGet)
	r.HandleFunc("/api/listings", adHandler.List).Methods(http.MethodGet)
	r.Handle("/api/listings", auth(listingHandler.Create)).Methods(http.MethodPost)
	r.Handle(listingPhotoPath, auth(listingHandler.UploadPhoto)).Methods(http.MethodPost)

	const listingPath = "/api/listings/{listing_id:[0-9]+}"
	r.Handle(listingPath, session.OptionalAuth(http.HandlerFunc(listingHandler.Get))).Methods(http.MethodGet)
	r.Handle(listingPath, auth(listingHandler.Update)).Methods(http.MethodPatch)
	r.Handle(listingPath, auth(listingHandler.Delete)).Methods(http.MethodDelete)
	r.Handle(listingPath+"/close", auth(listingHandler.Close)).Methods(http.MethodPost)

	r.Handle("/api/cart", auth(cartHandler.Get)).Methods(http.MethodGet)
	r.Handle("/api/cart/items", auth(cartHandler.Add)).Methods(http.MethodPost)
	r.Handle("/api/cart/items/bulk-add", auth(cartHandler.BulkAdd)).Methods(http.MethodPost)
	r.Handle("/api/cart/items/bulk-delete", auth(cartHandler.BulkRemove)).Methods(http.MethodPost)
	r.Handle("/api/cart/items/{listing_id:[0-9]+}", auth(cartHandler.Remove)).Methods(http.MethodDelete)

	r.Handle("/api/checkout/quote", verified(orderHandler.Quote)).Methods(http.MethodPost)
	r.Handle("/api/orders", verified(orderHandler.Create)).Methods(http.MethodPost)
	const orderPath = "/api/orders/{order_id:[0-9]+}"
	r.Handle(orderPath, auth(orderHandler.Get)).Methods(http.MethodGet)
	r.Handle(orderPath+"/payment", verified(orderHandler.Pay)).Methods(http.MethodPost)

	const paymentPath = testPaymentPath + "{token:[0-9a-f]{64}}"
	r.HandleFunc(paymentPath, orderHandler.TestPaymentPage).Methods(http.MethodGet)
	r.HandleFunc(paymentPath+"/success", orderHandler.TestPaymentResult(true)).Methods(http.MethodGet)
	r.HandleFunc(paymentPath+"/fail", orderHandler.TestPaymentResult(false)).Methods(http.MethodGet)

	r.PathPrefix("/uploads/").Handler(uploads(uploadDir)).Methods(http.MethodGet, http.MethodHead)

	timed := http.TimeoutHandler(r, requestTimeout, `{"error":"request timeout"}`)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == listingPhotoPath {
			r.ServeHTTP(w, req)
			return
		}
		timed.ServeHTTP(w, req)
	})

	return requestIDMiddleware(accessLogMiddleware(corsMiddleware(frontendOrigin, jsonContentType(handler))))
}
