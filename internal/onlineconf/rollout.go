package onlineconf

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
)

var ErrInvalidPercent = errors.New("rollout percent must be between 0 and 100")

type Rollout struct {
	Percent        int
	IncludeUserIDs []int32
	ExcludeUserIDs []int32
}

func (r Rollout) EnabledFor(scope, key string, userID int32) (bool, error) {
	if r.Percent < 0 || r.Percent > 100 {
		return false, ErrInvalidPercent
	}

	if userID <= 0 || slices.Contains(r.ExcludeUserIDs, userID) {
		return false, nil
	}

	if slices.Contains(r.IncludeUserIDs, userID) {
		return true, nil
	}

	seed := fmt.Sprintf("%q/%q/%d", scope, key, userID)
	hash := sha256.Sum256([]byte(seed))
	bucket := binary.BigEndian.Uint32(hash[:4]) % 100

	return int(bucket) < r.Percent, nil
}
