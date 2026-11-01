package usecase

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

const (
	payTimeout        = time.Hour
	deliveryDays      = 4
	maxOrderComment   = 200
	maxAddressText    = 300
	selfPickupHintEnd = "договоритесь с продавцом"
)

var (
	deliveryTimes  = []string{"10:00-14:00", "14:00-18:00", "18:00-22:00"}
	paymentMethods = []string{domain.PaymentCard, domain.PaymentSBP, domain.PaymentCOD}
	idempotencyKey = regexp.MustCompile(`^[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`)
	moscow         = time.FixedZone("MSK", 3*60*60)
)

type OrderUsecase struct {
	carts     CartRepository
	orders    OrderRepository
	addresses AddressRepository
	users     UserRepository
	tx        Transactor
	prices    domain.DeliveryPrices
	now       func() time.Time
}

func NewOrderUsecase(carts CartRepository, orders OrderRepository, addresses AddressRepository, users UserRepository,
	tx Transactor, prices domain.DeliveryPrices) *OrderUsecase {
	return &OrderUsecase{carts: carts, orders: orders, addresses: addresses, users: users, tx: tx, prices: prices, now: time.Now}
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

func (uc *OrderUsecase) Create(ctx context.Context, userID int32, in domain.OrderInput) (domain.Order, error) {
	if !idempotencyKey.MatchString(in.IdempotencyKey) {
		return domain.Order{}, domain.ErrInvalidIdempotency
	}
	ids, err := checkListingIDs(in.ListingIDs)
	if err != nil {
		return domain.Order{}, err
	}

	var orderID int32
	err = uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := uc.users.LockByID(ctx, userID); err != nil {
			return err
		}
		id, ok, err := uc.orders.FindByKey(ctx, userID, in.IdempotencyKey)
		if err != nil {
			return err
		}
		if ok {
			orderID = id
			return nil
		}
		if err := uc.orders.CancelOwnOverlapping(ctx, userID, ids); err != nil {
			return err
		}
		if err := uc.carts.LockListings(ctx, ids); err != nil {
			return err
		}
		entries, err := uc.carts.EntriesByIDs(ctx, userID, ids)
		if err != nil {
			return err
		}
		if err := checkAvailable(ids, entries); err != nil {
			return err
		}

		order, err := uc.buildOrder(ctx, userID, in, entries)
		if err != nil {
			return err
		}
		if orderID, err = uc.orders.Create(ctx, order, in.IdempotencyKey); err != nil {
			return err
		}
		if order.Status == domain.OrderPlaced {
			order.ID = orderID
			return uc.place(ctx, order)
		}
		return nil
	})
	if err != nil {
		return domain.Order{}, err
	}
	return uc.orders.Get(ctx, orderID)
}

func (uc *OrderUsecase) buildOrder(ctx context.Context, userID int32, in domain.OrderInput, entries []domain.CartEntry) (domain.Order, error) {
	items, total := orderItems(entries)
	sellers := quoteSellers(entries, uc.prices)

	bySeller, deliveryTotal, err := chooseDelivery(sellers, in.BySeller)
	if err != nil {
		return domain.Order{}, err
	}

	order := domain.Order{
		BuyerID:       userID,
		Items:         items,
		ItemsTotal:    total,
		BySeller:      bySeller,
		DeliveryTotal: deliveryTotal,
	}

	courier := slices.ContainsFunc(bySeller, func(s domain.SellerDelivery) bool { return s.Method == domain.DeliveryCourier })
	if courier {
		if err := uc.applyCourier(ctx, userID, in, &order); err != nil {
			return domain.Order{}, err
		}
	}

	comment := strings.TrimSpace(in.Comment)
	if utf8.RuneCountInString(comment) > maxOrderComment {
		return domain.Order{}, domain.ErrOrderCommentTooLong
	}
	order.Comment = comment

	name := normalizeFirstName(in.Recipient.FirstName)
	if err := checkFirstName(name); err != nil {
		return domain.Order{}, err
	}
	phone := normalizePhone(in.Recipient.Phone)
	if !isValidPhone(phone) {
		return domain.Order{}, domain.ErrInvalidPhone
	}
	order.Recipient = domain.Recipient{FirstName: name, Phone: phone}

	if !slices.Contains(paymentMethods, in.PaymentMethod) {
		return domain.Order{}, domain.ErrInvalidPaymentMethod
	}
	order.PaymentMethod = in.PaymentMethod
	if err := uc.startPayment(&order); err != nil {
		return domain.Order{}, err
	}
	payUntil := uc.now().Add(payTimeout)
	if order.Status == domain.OrderAwaitingPayment {
		order.PayUntil = &payUntil
	}
	return order, nil
}

func chooseDelivery(sellers []domain.QuoteSeller, chosen []domain.SellerDelivery) ([]domain.SellerDelivery, int64, error) {
	methods := make(map[int32]string, len(chosen))
	for _, c := range chosen {
		if _, dup := methods[c.SellerID]; dup {
			return nil, 0, domain.ErrDeliveryNotAvailable
		}
		methods[c.SellerID] = c.Method
	}
	if len(methods) != len(sellers) {
		return nil, 0, domain.ErrDeliveryNotAvailable
	}

	result := make([]domain.SellerDelivery, 0, len(sellers))
	var total int64
	for _, s := range sellers {
		method, ok := methods[s.SellerID]
		if !ok {
			return nil, 0, domain.ErrDeliveryNotAvailable
		}
		i := slices.IndexFunc(s.Options, func(o domain.DeliveryOption) bool { return o.Method == method && o.Available })
		if i < 0 {
			return nil, 0, domain.ErrDeliveryNotAvailable
		}
		result = append(result, domain.SellerDelivery{SellerID: s.SellerID, Method: method, Price: s.Options[i].Price})
		total += s.Options[i].Price
	}
	return result, total, nil
}

func (uc *OrderUsecase) applyCourier(ctx context.Context, userID int32, in domain.OrderInput, order *domain.Order) error {
	switch {
	case in.AddressID != nil:
		a, err := uc.addresses.GetForUpdate(ctx, userID, *in.AddressID)
		if errors.Is(err, domain.ErrAddressNotFound) {
			return domain.ErrDeliveryAddress
		}
		if err != nil {
			return err
		}
		text := addressText(a)
		order.AddressID, order.AddressText = &a.ID, &text
	case in.AddressText != nil:
		text := strings.TrimSpace(*in.AddressText)
		if text == "" || utf8.RuneCountInString(text) > maxAddressText {
			return domain.ErrDeliveryAddress
		}
		order.AddressText = &text
	default:
		return domain.ErrDeliveryAddress
	}

	if in.Date == nil || !slices.Contains(deliveryDates(uc.now()), *in.Date) {
		return domain.ErrDeliveryDate
	}
	if in.TimeSlot == nil || !slices.Contains(deliveryTimes, *in.TimeSlot) {
		return domain.ErrDeliveryTimeSlot
	}
	order.Date, order.TimeSlot = in.Date, in.TimeSlot
	return nil
}

func (uc *OrderUsecase) startPayment(order *domain.Order) error {
	if order.PaymentMethod == domain.PaymentCOD {
		order.Status, order.PaymentStatus = domain.OrderPlaced, domain.PaymentOnDelivery
		return nil
	}
	order.Status, order.PaymentStatus = domain.OrderAwaitingPayment, domain.PaymentPending
	return nil
}

func (uc *OrderUsecase) place(ctx context.Context, order domain.Order) error {
	if err := uc.orders.MarkSold(ctx, order.ID); err != nil {
		return err
	}
	ids := make([]int32, 0, len(order.Items))
	for _, it := range order.Items {
		if it.ListingID != nil {
			ids = append(ids, *it.ListingID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return uc.carts.Remove(ctx, order.BuyerID, ids)
}

func (uc *OrderUsecase) Get(ctx context.Context, userID, id int32) (domain.Order, error) {
	var order domain.Order
	err := uc.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		order, err = uc.ownForUpdate(ctx, userID, id)
		if err != nil {
			return err
		}
		return uc.settle(ctx, &order)
	})
	return order, err
}

func (uc *OrderUsecase) ownForUpdate(ctx context.Context, userID, id int32) (domain.Order, error) {
	order, err := uc.orders.GetForUpdate(ctx, id)
	if err != nil {
		return domain.Order{}, err
	}
	if order.BuyerID != userID {
		return domain.Order{}, domain.ErrOrderNotFound
	}
	return order, nil
}

func (uc *OrderUsecase) settle(ctx context.Context, order *domain.Order) error {
	if order.Status != domain.OrderAwaitingPayment {
		return nil
	}
	if order.PayUntil != nil && !uc.now().Before(*order.PayUntil) {
		order.Status = domain.OrderCancelled
		return uc.orders.UpdatePayment(ctx, *order)
	}
	return nil
}
