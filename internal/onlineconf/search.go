package onlineconf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	searchScope        = "search"
	searchPriceSortKey = "price_sort"
	maxSnapshotBytes   = 1 << 20
)

type SearchSnapshot struct {
	MaxQueryLength int32
	priceSort      Rollout
}

func (s SearchSnapshot) PriceSortEnabled(userID int32) bool {
	if userID <= 0 {
		return s.priceSort.Percent == 100
	}
	enabled, err := s.priceSort.EnabledFor(searchScope, searchPriceSortKey, userID)
	return err == nil && enabled
}

func (s SearchSnapshot) PriceSortDependsOnUser() bool {
	switch s.priceSort.Percent {
	case 0:
		return len(s.priceSort.IncludeUserIDs) > 0
	case 100:
		return len(s.priceSort.ExcludeUserIDs) > 0
	default:
		return true
	}
}

type SearchCache struct {
	snapshotURL string
	client      http.Client
	current     atomic.Pointer[SearchSnapshot]
}

func NewSearchCache(snapshotURL string) *SearchCache {
	cache := &SearchCache{
		snapshotURL: snapshotURL,
		client:      http.Client{Timeout: 3 * time.Second},
	}
	initial := SearchSnapshot{}
	if snapshotURL == "" {
		initial.priceSort.Percent = 100
	}
	cache.current.Store(&initial)
	return cache
}

func (c *SearchCache) Snapshot() SearchSnapshot {
	return *c.current.Load()
}

func (c *SearchCache) Run(ctx context.Context, interval time.Duration) {
	if c.snapshotURL == "" {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if err := c.refresh(ctx); err != nil && ctx.Err() == nil {
			slog.WarnContext(ctx, "refresh OnlineConf search settings", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *SearchCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.snapshotURL, nil)
	if err != nil {
		return errors.New("invalid OnlineConf snapshot URL")
	}
	query := req.URL.Query()
	query.Set("scope", searchScope)
	req.URL.RawQuery = query.Encode()
	resp, err := c.client.Do(req)
	if err != nil {
		return errors.New("OnlineConf request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("OnlineConf returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSnapshotBytes+1))
	if err != nil {
		return errors.New("read OnlineConf snapshot")
	}
	if len(body) > maxSnapshotBytes {
		return errors.New("OnlineConf snapshot is too large")
	}
	snapshot, err := decodeSearchSnapshot(body)
	if err != nil {
		return err
	}
	c.current.Store(&snapshot)
	return nil
}

func decodeSearchSnapshot(body []byte) (SearchSnapshot, error) {
	var entries []struct {
		Scope string          `json:"scope"`
		Name  string          `json:"name"`
		Kind  string          `json:"kind"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(body, &entries); err != nil || entries == nil {
		return SearchSnapshot{}, errors.New("invalid OnlineConf snapshot")
	}

	snapshot := SearchSnapshot{priceSort: Rollout{Percent: 100}}
	for _, entry := range entries {
		if entry.Scope != searchScope {
			continue
		}
		switch {
		case entry.Name == "max_query_length" && entry.Kind == "config":
			var limit int32
			if err := json.Unmarshal(entry.Value, &limit); err != nil || limit <= 0 {
				return SearchSnapshot{}, errors.New("invalid max_query_length")
			}
			snapshot.MaxQueryLength = limit
		case entry.Name == searchPriceSortKey && entry.Kind == "flag":
			flag, err := decodePriceSort(entry.Value)
			if err != nil {
				return SearchSnapshot{}, err
			}
			snapshot.priceSort = flag
		}
	}
	return snapshot, nil
}

func decodePriceSort(raw json.RawMessage) (Rollout, error) {
	var enabled *bool
	if err := json.Unmarshal(raw, &enabled); err == nil && enabled != nil {
		if *enabled {
			return Rollout{Percent: 100}, nil
		}
		return Rollout{Percent: 0}, nil
	}

	var payload struct {
		Percent     *int    `json:"rollout_percent"`
		IncludedIDs []int32 `json:"included_user_ids"`
		ExcludedIDs []int32 `json:"excluded_user_ids"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.Percent == nil ||
		*payload.Percent < 0 || *payload.Percent > 100 {
		return Rollout{}, errors.New("invalid price_sort rollout percentage")
	}
	for _, ids := range [][]int32{payload.IncludedIDs, payload.ExcludedIDs} {
		for _, id := range ids {
			if id <= 0 {
				return Rollout{}, errors.New("invalid price_sort user ID")
			}
		}
	}
	return Rollout{Percent: *payload.Percent, IncludeUserIDs: payload.IncludedIDs, ExcludeUserIDs: payload.ExcludedIDs}, nil
}
