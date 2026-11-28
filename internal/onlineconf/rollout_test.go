package onlineconf

import (
	"errors"
	"testing"
)

func TestRolloutEnabledFor(t *testing.T) {
	for _, tt := range []struct {
		name string
		rule Rollout
		user int32
		want bool
	}{
		{"zero", Rollout{Percent: 0}, 7, false},
		{"full", Rollout{Percent: 100}, 7, true},
		{"before threshold", Rollout{Percent: 8}, 7, false},
		{"included at threshold", Rollout{Percent: 9}, 7, true},
		{"still included at 50 percent", Rollout{Percent: 50}, 7, true},
		{"another user outside 50 percent", Rollout{Percent: 50}, 1, false},
		{"forced on", Rollout{IncludeUserIDs: []int32{7}}, 7, true},
		{"forced off", Rollout{Percent: 100, ExcludeUserIDs: []int32{7}}, 7, false},
		{"both lists", Rollout{IncludeUserIDs: []int32{7}, ExcludeUserIDs: []int32{7}}, 7, false},
		{"guest", Rollout{Percent: 100}, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.rule.EnabledFor("search", "new_sort", tt.user)
			if err != nil || got != tt.want {
				t.Fatalf("got (%v, %v), want (%v, nil)", got, err, tt.want)
			}
		})
	}
}

func TestRolloutInvalidPercent(t *testing.T) {
	for _, percent := range []int{-1, 101} {
		got, err := (Rollout{Percent: percent}).EnabledFor("search", "new_sort", 7)
		if got || !errors.Is(err, ErrInvalidPercent) {
			t.Fatalf("percent %d: got (%v, %v)", percent, got, err)
		}
	}
}
