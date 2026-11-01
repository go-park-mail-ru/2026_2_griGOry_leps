package usecase

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	deliveryDays      = 4
	selfPickupHintEnd = "договоритесь с продавцом"
)

var (
	deliveryTimes  = []string{"10:00-14:00", "14:00-18:00", "18:00-22:00"}
	paymentMethods = []string{domain.PaymentCard, domain.PaymentSBP, domain.PaymentCOD}
	moscow         = time.FixedZone("MSK", 3*60*60)
)

type OrderUsecase struct {
	carts     CartRepository
	addresses AddressRepository
	users     UserRepository
	prices    domain.DeliveryPrices
	now       func() time.Time
}

func NewOrderUsecase(carts CartRepository, addresses AddressRepository, users UserRepository,
	prices domain.DeliveryPrices) *OrderUsecase {
	return &OrderUsecase{carts: carts, addresses: addresses, users: users, prices: prices, now: time.Now}
}

func (uc *OrderUsecase) Quote(ctx context.Context, userID int32, ids []int32) (domain.CheckoutQuote, error) {
	ids, err := checkListingIDs(ids)
	if err != nil {
		return domain.CheckoutQuote{}, err
	}

	entries, err := uc.carts.EntriesByIDs(ctx, userID, ids)
	if err != nil {
		return domain.CheckoutQuote{}, err
	}
	if err := checkAvailable(ids, entries); err != nil {
		return domain.CheckoutQuote{}, err
	}

	user, err := uc.users.GetByID(ctx, userID)
	if err != nil {
		return domain.CheckoutQuote{}, err
	}
	addresses, err := uc.addresses.List(ctx, userID)
	if err != nil {
		return domain.CheckoutQuote{}, err
	}

	items, total := orderItems(entries)
	q := domain.CheckoutQuote{
		Items:      items,
		ItemsTotal: total,
		Sellers:    quoteSellers(entries, uc.prices),
		Addresses:  make([]domain.QuoteAddress, 0, len(addresses)),
		Dates:      deliveryDates(uc.now()),
		Times:      deliveryTimes,
		Recipient:  domain.Recipient{FirstName: user.FirstName, Phone: user.Phone},
		Payments:   paymentMethods,
	}
	for _, a := range addresses {
		label := "Адрес"
		if a.IsDefault {
			label = "Основной адрес"
		}
		q.Addresses = append(q.Addresses, domain.QuoteAddress{ID: a.ID, Label: label, Text: addressText(a), IsDefault: a.IsDefault})
	}
	return q, nil
}

func checkAvailable(ids []int32, entries []domain.CartEntry) error {
	found := make(map[int32]bool, len(entries))
	for _, e := range entries {
		if e.UnavailableReason() == "" {
			found[e.ListingID] = true
		}
	}
	var missing []int32
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return &domain.ItemsUnavailableError{ListingIDs: missing}
	}
	return nil
}

func orderItems(entries []domain.CartEntry) ([]domain.OrderItem, int64) {
	items := make([]domain.OrderItem, 0, len(entries))
	var total int64
	for _, e := range entries {
		id := e.ListingID
		var price int64
		if e.Price != nil {
			price = *e.Price
		}
		total += price
		items = append(items, domain.OrderItem{ListingID: &id, SellerID: e.Seller.ID, Title: e.Title, Price: price, ImageURL: e.ImageURL})
	}
	return items, total
}

func quoteSellers(entries []domain.CartEntry, prices domain.DeliveryPrices) []domain.QuoteSeller {
	var sellers []domain.QuoteSeller
	index := make(map[int32]int)
	ships := make(map[int32]bool)
	addresses := make(map[int32][]string)
	for _, e := range entries {
		i, ok := index[e.Seller.ID]
		if !ok {
			i = len(sellers)
			index[e.Seller.ID] = i
			sellers = append(sellers, domain.QuoteSeller{SellerID: e.Seller.ID, FirstName: e.Seller.FirstName})
			ships[e.Seller.ID] = true
		}
		sellers[i].ItemsCount++
		ships[e.Seller.ID] = ships[e.Seller.ID] && e.HasDelivery
		if e.Address != nil && *e.Address != "" && !slices.Contains(addresses[e.Seller.ID], *e.Address) {
			addresses[e.Seller.ID] = append(addresses[e.Seller.ID], *e.Address)
		}
	}
	for i := range sellers {
		id := sellers[i].SellerID
		sellers[i].Options = deliveryOptions(ships[id], addresses[id], prices)
	}
	return sellers
}

func deliveryOptions(ships bool, addresses []string, prices domain.DeliveryPrices) []domain.DeliveryOption {
	courierHint, pickupHint := "С завтрашнего дня, интервал 4 часа", "Через 2 дня в ближайший пункт выдачи"
	if !ships {
		courierHint, pickupHint = "Продавец не отправляет доставкой", "Продавец не отправляет доставкой"
	}
	selfHint := selfPickupHintEnd
	if len(addresses) > 0 {
		selfHint = strings.Join(addresses, "; ") + " · " + selfPickupHintEnd
	}
	return []domain.DeliveryOption{
		{Method: domain.DeliveryCourier, Price: prices.Courier, Available: ships, Hint: courierHint},
		{Method: domain.DeliveryPickupPoint, Price: prices.PickupPoint, Available: ships, Hint: pickupHint},
		{Method: domain.DeliverySelfPickup, Price: 0, Available: true, Hint: selfHint},
	}
}

func deliveryDates(now time.Time) []string {
	today := now.In(moscow)
	dates := make([]string, 0, deliveryDays)
	for i := 1; i <= deliveryDays; i++ {
		dates = append(dates, today.AddDate(0, 0, i).Format(time.DateOnly))
	}
	return dates
}

func addressText(a domain.Address) string {
	text := a.Street + ", " + a.House
	if a.Comment != "" {
		text += ", " + a.Comment
	}
	return text
}
