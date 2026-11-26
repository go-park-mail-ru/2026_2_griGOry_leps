package usecase

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

func uniqueIDs(ids []int32) []int32 {
	seen := make(map[int32]struct{}, len(ids))
	unique := make([]int32, 0, len(ids))
	for _, id := range ids {
		if _, dup := seen[id]; !dup {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	return unique
}

// userCards загружает карточки одним вызовом; если хоть одного ID нет в ответе, возвращает ErrUserMissing.
// Повторы схлопываются, порядок ID в запросе — по первому появлению.
func userCards(ctx context.Context, users UserDirectory, ids ...int32) (map[int32]domain.UserCard, error) {
	unique := uniqueIDs(ids)
	cards, err := users.Users(ctx, unique)
	if err != nil {
		return nil, err
	}
	for _, id := range unique {
		if _, ok := cards[id]; !ok {
			return nil, domain.ErrUserMissing
		}
	}
	return cards, nil
}

// fillSeller дополняет продавца публичными данными из auth; рейтинг и счётчик остаются из ads.
func fillSeller(ctx context.Context, users UserDirectory, s *domain.Seller) error {
	cards, err := userCards(ctx, users, s.ID)
	if err != nil {
		return err
	}
	c := cards[s.ID]
	s.FirstName, s.Avatar, s.CreatedAt = c.FirstName, c.Avatar, c.CreatedAt
	return nil
}

// fillCartSellers дополняет продавцов позиций корзины одним вызовом справочника.
// extra — дополнительные ID, которые нужны вызывающему (например, покупатель); карточки возвращаются все.
func fillCartSellers(ctx context.Context, users UserDirectory, entries []domain.CartEntry, extra ...int32) (map[int32]domain.UserCard, error) {
	ids := make([]int32, 0, len(entries)+len(extra))
	for _, e := range entries {
		ids = append(ids, e.Seller.ID)
	}
	ids = append(ids, extra...)
	if len(ids) == 0 {
		return nil, nil
	}
	cards, err := userCards(ctx, users, ids...)
	if err != nil {
		return nil, err
	}
	for i := range entries {
		c := cards[entries[i].Seller.ID]
		entries[i].Seller.FirstName, entries[i].Seller.Avatar = c.FirstName, c.Avatar
	}
	return cards, nil
}

// requireSeller возвращает ErrSellerNotFound, если auth не знает такого пользователя.
func requireSeller(ctx context.Context, users UserDirectory, id int32) (domain.UserCard, error) {
	cards, err := users.Users(ctx, []int32{id})
	if err != nil {
		return domain.UserCard{}, err
	}
	card, ok := cards[id]
	if !ok {
		return domain.UserCard{}, domain.ErrSellerNotFound
	}
	return card, nil
}
