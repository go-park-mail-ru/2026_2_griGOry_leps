package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/onlineconf"
	"strings"
	"unicode/utf8"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type OnlineconfUsecase struct {
	entries OnlineconfRepository
}

func NewOnlineconfUsecase(entries OnlineconfRepository) *OnlineconfUsecase {
	return &OnlineconfUsecase{entries: entries}
}

func validOnlineconfScope(scope string) bool {
	switch scope {
	case "frontend", "auth", "ads", "search":
		return true
	default:
		return false
	}
}

func validOnlineconfKind(kind string) bool {
	return kind == "config" || kind == "flag"
}

func validOnlineconfName(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.RuneCountInString(name) <= 255
}

func validOnlineconfFlag(raw json.RawMessage) bool {
	value := bytes.TrimSpace(raw)
	if string(value) == "true" || string(value) == "false" {
		return true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(value, &fields) != nil || len(fields) != 3 {
		return false
	}
	var percentage *int
	if json.Unmarshal(fields["rollout_percent"], &percentage) != nil || percentage == nil || *percentage < 0 || *percentage > 100 {
		return false
	}
	for _, key := range []string{"included_user_ids", "excluded_user_ids"} {
		var ids []int32
		if json.Unmarshal(fields[key], &ids) != nil || ids == nil {
			return false
		}
		for _, id := range ids {
			if id <= 0 {
				return false
			}
		}
	}
	return true
}

func (uc *OnlineconfUsecase) Create(
	ctx context.Context, actor int32, entry domain.OnlineconfEntry,
) (domain.OnlineconfEntry, error) {
	if !validOnlineconfScope(entry.Scope) || !validOnlineconfName(entry.Name) ||
		!validOnlineconfKind(entry.Kind) || !json.Valid(entry.Value) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}

	if entry.Kind == "flag" && !validOnlineconfFlag(entry.Value) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}
	if entry.Scope == "search" && entry.Name == "max_query_length" {
		var length int64
		if entry.Kind != "config" || json.Unmarshal(entry.Value, &length) != nil || length <= 0 {
			return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
		}
	}
	return uc.entries.Create(ctx, actor, entry)
}

func (uc *OnlineconfUsecase) Update(
	ctx context.Context, actor int32, id int, patch domain.OnlineconfPatch,
) (domain.OnlineconfEntry, error) {
	if id <= 0 || (patch.Scope == nil && patch.Name == nil && patch.Value == nil) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}
	if patch.Scope != nil && !validOnlineconfScope(*patch.Scope) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}
	if patch.Name != nil && !validOnlineconfName(*patch.Name) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}
	if patch.Value != nil && !json.Valid(patch.Value) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}

	return uc.entries.Update(ctx, actor, id, patch)
}

func (uc *OnlineconfUsecase) Search(
	ctx context.Context, filter domain.OnlineconfFilter,
) ([]domain.OnlineconfEntry, error) {
	if filter.Scope != nil && !validOnlineconfScope(*filter.Scope) {
		return nil, domain.ErrInvalidOnlineconf
	}
	if filter.Kind != nil && !validOnlineconfKind(*filter.Kind) {
		return nil, domain.ErrInvalidOnlineconf
	}
	return uc.entries.Search(ctx, filter)
}

func (uc *OnlineconfUsecase) History(ctx context.Context, id int) ([]domain.OnlineconfChange, error) {
	return uc.entries.History(ctx, id)
}

func (uc *OnlineconfUsecase) CheckFlag(ctx context.Context, id int, userID int32) (bool, error) {
	if id <= 0 || userID <= 0 {
		return false, domain.ErrInvalidOnlineconf
	}
	entry, err := uc.entries.Get(ctx, id)
	if err != nil {
		return false, err
	}
	if entry.Kind != "flag" {
		return false, domain.ErrInvalidOnlineconf
	}
	rule, err := onlineconf.DecodeRollout(entry.Value)
	if err != nil {
		return false, err
	}
	return rule.EnabledFor(entry.Scope, entry.Name, userID)
}
