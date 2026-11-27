package repository

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// schemaQueries описывают структуру схемы так, чтобы её можно было сравнить между базами.
var schemaQueries = map[string]string{
	"columns": `
		SELECT table_name || '.' || column_name || ' ' || data_type || ' null=' || is_nullable ||
		       ' default=' || COALESCE(column_default, '') || ' identity=' || is_identity ||
		       ' len=' || COALESCE(character_maximum_length, 0) || ' prec=' || COALESCE(numeric_precision, 0) ||
		       ' scale=' || COALESCE(numeric_scale, 0)
		FROM information_schema.columns WHERE table_schema = current_schema()`,
	"indexes": `
		SELECT tablename || ' ' || indexname || ' ' || replace(indexdef, current_schema() || '.', '')
		FROM pg_indexes WHERE schemaname = current_schema()`,
	"constraints": `
		SELECT conrelid::regclass::text || ' ' || conname || ' ' || contype::text || ' ' || pg_get_constraintdef(oid)
		FROM pg_constraint WHERE connamespace = current_schema()::regnamespace`,
	"triggers": `
		SELECT event_object_table || ' ' || trigger_name || ' ' || action_timing || ' ' || event_manipulation || ' ' || action_statement
		FROM information_schema.triggers WHERE trigger_schema = current_schema()`,
	"functions": `
		SELECT proname || ' ' || md5(prosrc) FROM pg_proc WHERE pronamespace = current_schema()::regnamespace`,
	"tables": `
		SELECT table_name FROM information_schema.tables WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'`,
}

func schemaFacts(t *testing.T, db *sql.DB) map[string][]string {
	t.Helper()
	facts := make(map[string][]string, len(schemaQueries))
	for name, query := range schemaQueries {
		rows, err := db.QueryContext(context.Background(), query)
		require.NoError(t, err, name)
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			facts[name] = append(facts[name], line)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		slices.Sort(facts[name])
		facts[name] = slices.Compact(facts[name])
	}
	return facts
}

// dropCrossServiceForeignKeys убирает внешние ключи таблиц ads на user_account: после разделения
// их нет. Ключи внутри auth (session, password_reset, email_verification) остаются.
func dropCrossServiceForeignKeys(lines, adsTables []string) []string {
	return slices.DeleteFunc(slices.Clone(lines), func(line string) bool {
		table, _, _ := strings.Cut(line, " ")
		return slices.Contains(adsTables, table) &&
			strings.Contains(line, " f FOREIGN KEY") && strings.Contains(line, "REFERENCES user_account(")
	})
}

// Объединение baseline auth и ads должно совпасть с историческими миграциями 1-14,
// за вычетом внешних ключей на user_account, которые пересекают границу сервисов.
func TestBaselineMatchesLegacy(t *testing.T) {
	legacy := schemaFacts(t, newSchemaPool(t, "legacy"))
	auth := schemaFacts(t, newSchemaPool(t, "auth"))
	ads := schemaFacts(t, newSchemaPool(t, "ads"))

	for _, table := range auth["tables"] {
		require.NotContains(t, ads["tables"], table, "table is owned by both services")
	}

	for name := range schemaQueries {
		want := legacy[name]
		if name == "constraints" {
			want = dropCrossServiceForeignKeys(want, ads["tables"])
		}
		got := slices.Concat(auth[name], ads[name])
		slices.Sort(got)
		got = slices.Compact(got)
		require.Equal(t, want, got, fmt.Sprintf("schema facts %q differ from legacy migrations", name))
	}
}

// Справочные данные baseline совпадают с теми, что создают исторические миграции.
func TestBaselineSeedsMatchLegacy(t *testing.T) {
	ctx := context.Background()
	legacy := newSchemaPool(t, "legacy")
	auth := newSchemaPool(t, "auth")
	ads := newSchemaPool(t, "ads")

	rows := func(db *sql.DB, query string) []string {
		r, err := db.QueryContext(ctx, query)
		require.NoError(t, err)
		defer func() { _ = r.Close() }()
		var out []string
		for r.Next() {
			var s string
			require.NoError(t, r.Scan(&s))
			out = append(out, s)
		}
		require.NoError(t, r.Err())
		return out
	}
	const cities = `SELECT name FROM city ORDER BY name`
	const categories = `
		SELECT c.slug || '|' || c.name || '|' || c.delivery_allowed || '|' || COALESCE(p.slug, '')
		FROM category c LEFT JOIN category p ON p.category_id = c.parent_id ORDER BY c.slug`
	require.Equal(t, rows(legacy, cities), rows(auth, cities))
	require.Equal(t, rows(legacy, categories), rows(ads, categories))
}

func TestCrossServiceForeignKeys(t *testing.T) {
	lines := []string{
		"ad ad_category_id_fkey f FOREIGN KEY (category_id) REFERENCES category(category_id) ON DELETE SET NULL",
		"ad ad_user_id_fkey f FOREIGN KEY (user_id) REFERENCES user_account(user_id) ON DELETE CASCADE",
		"session session_user_id_fkey f FOREIGN KEY (user_id) REFERENCES user_account(user_id) ON DELETE CASCADE",
		"orders orders_buyer_id_fkey f FOREIGN KEY (buyer_id) REFERENCES user_account(user_id) ON DELETE CASCADE",
	}
	adsTables := []string{"ad", "orders", "category"}

	got := dropCrossServiceForeignKeys(lines, adsTables)

	require.Equal(t, []string{lines[0], lines[2]}, got)
}
