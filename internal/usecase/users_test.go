package usecase

import (
	"context"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

// fakeDirectory — ручная замена UserDirectory. Для ID без явной карточки отдаёт пустую карточку с этим ID,
// чтобы тесты, не проверяющие данные пользователя, не зависели от справочника. missing исключает ID из ответа.
type fakeDirectory struct {
	cards   map[int32]domain.UserCard
	missing map[int32]bool
	err     error
	calls   [][]int32
}

func newFakeDirectory(cards ...domain.UserCard) *fakeDirectory {
	f := &fakeDirectory{cards: map[int32]domain.UserCard{}, missing: map[int32]bool{}}
	for _, c := range cards {
		f.cards[c.ID] = c
	}
	return f
}

func (f *fakeDirectory) Users(_ context.Context, ids []int32) (map[int32]domain.UserCard, error) {
	f.calls = append(f.calls, append([]int32(nil), ids...))
	if f.err != nil {
		return nil, f.err
	}
	out := map[int32]domain.UserCard{}
	for _, id := range ids {
		if f.missing[id] {
			continue
		}
		if c, ok := f.cards[id]; ok {
			out[id] = c
		} else {
			out[id] = domain.UserCard{ID: id}
		}
	}
	return out, nil
}
