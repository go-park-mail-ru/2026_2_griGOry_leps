package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var start = time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)

func TestAllow_BlocksAfterLimit(t *testing.T) {
	l := New(3, time.Minute)

	for i := 0; i < 3; i++ {
		ok, _ := l.Allow("1.2.3.4", start.Add(time.Duration(i)*time.Second))
		require.True(t, ok)
	}

	ok, retryAfter := l.Allow("1.2.3.4", start.Add(20*time.Second))
	require.False(t, ok)
	require.Equal(t, 40*time.Second, retryAfter)
}

func TestAllow_ResetsAfterWindow(t *testing.T) {
	l := New(1, time.Minute)

	ok, _ := l.Allow("1.2.3.4", start)
	require.True(t, ok)
	ok, _ = l.Allow("1.2.3.4", start.Add(59*time.Second))
	require.False(t, ok)

	ok, _ = l.Allow("1.2.3.4", start.Add(time.Minute))
	require.True(t, ok)
}

func TestAllow_KeysAreIndependent(t *testing.T) {
	l := New(1, time.Minute)

	ok, _ := l.Allow("1.2.3.4", start)
	require.True(t, ok)
	ok, _ = l.Allow("5.6.7.8", start)
	require.True(t, ok)
}

func TestAllow_BlockedRequestsDoNotExtendWindow(t *testing.T) {
	l := New(1, time.Minute)

	l.Allow("1.2.3.4", start)
	for i := 1; i < 60; i++ {
		l.Allow("1.2.3.4", start.Add(time.Duration(i)*time.Second))
	}

	ok, _ := l.Allow("1.2.3.4", start.Add(time.Minute))
	require.True(t, ok)
}

func TestAllow_SweepsExpiredCounters(t *testing.T) {
	l := New(1, time.Minute)

	l.Allow("1.2.3.4", start)
	l.Allow("5.6.7.8", start)
	l.Allow("9.9.9.9", start.Add(2*time.Minute))

	require.Len(t, l.counters, 1)
}
