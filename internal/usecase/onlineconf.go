package usecase

import (
	"bytes"
	"context"
	"encoding/json"
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

func (uc *OnlineconfUsecase) Create(
	ctx context.Context, entry domain.OnlineconfEntry,
) (domain.OnlineconfEntry, error) {
	if !validOnlineconfScope(entry.Scope) || !validOnlineconfName(entry.Name) ||
		!validOnlineconfKind(entry.Kind) || !json.Valid(entry.Value) {
		return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
	}

	if entry.Kind == "flag" {
		value := string(bytes.TrimSpace(entry.Value))
		if value != "true" && value != "false" {
			return domain.OnlineconfEntry{}, domain.ErrInvalidOnlineconf
		}
	}

	return uc.entries.Create(ctx, entry)
}

func (uc *OnlineconfUsecase) Update(
	ctx context.Context, id int, patch domain.OnlineconfPatch,
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

	return uc.entries.Update(ctx, id, patch)
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
