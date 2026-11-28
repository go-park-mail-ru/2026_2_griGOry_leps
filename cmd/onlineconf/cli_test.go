package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/go-park-mail-ru/2026_2_griGOry_leps/internal/domain"
)

type cliTestKey struct{}

func TestRunCLI_Granted(t *testing.T) {
	var stdout, stderr bytes.Buffer
	ctx := context.WithValue(context.Background(), cliTestKey{}, "cli")
	calls := 0
	grant := func(gotCtx context.Context, id int32) (bool, error) {
		calls++
		require.Equal(t, ctx, gotCtx)
		require.Equal(t, int32(7), id)
		return true, nil
	}
	code := runCLI(ctx, []string{"grant-admin", "--user-id", "7"}, grant, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.Equal(t, 1, calls)
	require.Equal(t, "granted user 7\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunCLI_AlreadyAdmin(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grant := func(context.Context, int32) (bool, error) { return false, nil }
	code := runCLI(context.Background(), []string{"grant-admin", "--user-id=7"}, grant, &stdout, &stderr)
	require.Equal(t, 0, code)
	require.Equal(t, "user 7 is already an admin\n", stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunCLI_GrantError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grant := func(context.Context, int32) (bool, error) { return false, errors.New("db down") }
	code := runCLI(context.Background(), []string{"grant-admin", "--user-id", "7"}, grant, &stdout, &stderr)
	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "db down\n")
}

func TestRunCLI_InvalidUserID(t *testing.T) {
	for _, args := range [][]string{
		{"grant-admin"}, {"grant-admin", "--user-id"},
		{"grant-admin", "--user-id", "0"}, {"grant-admin", "--user-id", "-5"},
		{"grant-admin", "--user-id", "abc"}, {"grant-admin", "--user-id", "2147483648"},
		{"grant-admin", "--user-id", "7", "extra"}, {"grant-admin", "--unknown"},
		{"grant-admin", "extra", "--user-id", "7"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			grant := func(context.Context, int32) (bool, error) {
				t.Error("grant called with invalid args")
				return false, nil
			}
			code := runCLI(context.Background(), args, grant, &stdout, &stderr)
			require.Equal(t, 1, code)
			require.Empty(t, stdout.String())
			require.NotEmpty(t, stderr.String())
		})
	}
}

func TestRunCLI_UnknownCommand(t *testing.T) {
	for _, args := range [][]string{nil, {"revoke"}} {
		var stdout, stderr bytes.Buffer
		grant := func(context.Context, int32) (bool, error) { t.Error("grant called"); return false, nil }
		code := runCLI(context.Background(), args, grant, &stdout, &stderr)
		require.Equal(t, 2, code)
		require.Contains(t, stderr.String(), "usage: onlineconf grant-admin --user-id N")
		require.Empty(t, stdout.String())
	}
}

func TestRunCLI_MaxUserID(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grant := func(_ context.Context, id int32) (bool, error) {
		require.Equal(t, int32(2147483647), id)
		return true, nil
	}
	require.Equal(t, 0, runCLI(context.Background(), []string{"grant-admin", "--user-id", "2147483647"}, grant, &stdout, &stderr))
	require.Equal(t, "granted user 2147483647\n", stdout.String())
}

func TestRequireDatabaseURL(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) { v, ok := values[key]; return v, ok }
	}
	url, err := requireDatabaseURL(lookup(map[string]string{"DATABASE_URL": "postgres://db/goget"}))
	require.NoError(t, err)
	require.Equal(t, "postgres://db/goget", url)

	for _, values := range []map[string]string{{}, {"DATABASE_URL": ""}} {
		_, err := requireDatabaseURL(lookup(values))
		require.EqualError(t, err, "DATABASE_URL must be set")
	}
}

func TestRunCLI_UnknownUser(t *testing.T) {
	var stdout, stderr bytes.Buffer
	grant := func(context.Context, int32) (bool, error) {
		return false, fmt.Errorf("grant onlineconf admin: %w", domain.ErrUserNotFound)
	}
	code := runCLI(context.Background(), []string{"grant-admin", "--user-id", "7"}, grant, &stdout, &stderr)
	require.Equal(t, 1, code)
	require.Empty(t, stdout.String())
	require.Equal(t, "user 7 not found\n", stderr.String())
}
