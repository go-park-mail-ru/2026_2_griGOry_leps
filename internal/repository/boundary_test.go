package repository

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	adsOnlyFiles  = []string{"ad_repo", "listing", "cart", "seller", "address", "order", "moderation"}
	authOnlyFiles = []string{"user", "session", "password_reset", "email_verification", "city"}
	fromAuthTable = regexp.MustCompile(`(?i)\b(from|join)\s+(user_account|session|password_reset|email_verification|city)\b`)
	fromAdsTable  = regexp.MustCompile(`(?i)\b(from|join)\s+(ad|review|orders|cart|cart_item|address|category)\b`)
)

// crossesBoundary сообщает, обращается ли SQL к таблицам другого сервиса.
func crossesBoundary(sql string, foreign *regexp.Regexp) bool { return foreign.MatchString(sql) }

func TestCrossesBoundary(t *testing.T) {
	require.True(t, crossesBoundary("SELECT 1 FROM ad a JOIN user_account u ON u.user_id = a.user_id", fromAuthTable))
	require.True(t, crossesBoundary("LEFT JOIN city c ON c.city_id = u.city_id", fromAuthTable))
	require.False(t, crossesBoundary("SELECT COALESCE(a.city, '') FROM ad a", fromAuthTable))
	require.True(t, crossesBoundary("LEFT JOIN LATERAL (SELECT 1 FROM review rv) r ON true", fromAdsTable))
}

// Репозитории сервиса не должны читать таблицы другого сервиса: эти данные приходят по gRPC.
func TestRepositoriesStayInsideServiceBoundary(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		require.NoError(t, err)
		base := strings.TrimSuffix(strings.TrimSuffix(file, "_queries.go"), ".go")
		switch {
		case hasPrefixIn(base, adsOnlyFiles):
			require.False(t, crossesBoundary(string(src), fromAuthTable), "%s (ads) reads an auth table", file)
		case hasPrefixIn(base, authOnlyFiles):
			require.False(t, crossesBoundary(string(src), fromAdsTable), "%s (auth) reads an ads table", file)
		}
	}
}

func hasPrefixIn(name string, prefixes []string) bool {
	for _, p := range prefixes {
		if name == p || strings.HasPrefix(name, p+"_") {
			return true
		}
	}
	return false
}
