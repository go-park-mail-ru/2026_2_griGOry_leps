//go:build integration

package repository

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
)

type splitEnv struct {
	admin                      *sql.DB
	source, auth, ads          *sql.DB
	sourceURL, authURL, adsURL string
}

// newSplitEnv создаёт исходную базу с историческими миграциями, сидом и данными заказа плюс две пустые целевые базы.
func newSplitEnv(t *testing.T, goose int) *splitEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("integration test requires TEST_DATABASE_URL")
	}
	for _, tool := range []string{"bash", "psql", "pg_dump", "goose"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("split-db test requires %s", tool)
		}
	}
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	admin := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = admin.Close() })
	skipIfDumpIsOlderThanServer(t, admin)

	var token [6]byte
	_, err = rand.Read(token[:])
	require.NoError(t, err)
	suffix := hex.EncodeToString(token[:])
	e := &splitEnv{admin: admin}
	open := func(role string) (*sql.DB, string) {
		name := "split_" + role + "_" + suffix
		_, err := admin.Exec(`CREATE DATABASE ` + name)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + name + ` WITH (FORCE)`) })
		u, err := url.Parse(dsn)
		require.NoError(t, err)
		u.Path = "/" + name
		c, err := pgx.ParseConfig(u.String())
		require.NoError(t, err)
		db := stdlib.OpenDB(*c)
		t.Cleanup(func() { _ = db.Close() })
		return db, u.String()
	}
	e.source, e.sourceURL = open("src")
	e.auth, e.authURL = open("auth")
	e.ads, e.adsURL = open("ads")

	e.applyDir(t, e.source, "legacy")
	seed, err := os.ReadFile("../../db/legacy/seed.sql")
	require.NoError(t, err)
	_, err = e.source.Exec(string(seed))
	require.NoError(t, err)
	_, err = e.source.Exec(`
		CREATE TABLE goose_db_version (id serial PRIMARY KEY, version_id bigint NOT NULL, is_applied boolean NOT NULL, tstamp timestamp DEFAULT now());
		INSERT INTO goose_db_version (version_id, is_applied) VALUES (` + strconv.Itoa(goose) + `, true);
		INSERT INTO address (user_id, city, street, house) VALUES (2, 'Москва', 'Тверская', '1');
		INSERT INTO orders (buyer_id, idempotency_key, status, payment_method, payment_status, recipient_name, recipient_phone, items_total, delivery_total)
		VALUES (2, gen_random_uuid(), 'placed', 'cod', 'on_delivery', 'Ольга', '+79990000002', 100, 0);
		INSERT INTO order_item (order_id, ad_id, seller_id, title, price) SELECT 1, min(id), 1, 'Велосипед', 100 FROM ad;`)
	require.NoError(t, err)
	return e
}

func (e *splitEnv) applyDir(t *testing.T, db *sql.DB, dir string) {
	t.Helper()
	files, err := filepath.Glob("../../migrations/" + dir + "/*.sql")
	require.NoError(t, err)
	sort.Slice(files, func(i, j int) bool { return migrationNumber(t, files[i]) < migrationNumber(t, files[j]) })
	for _, file := range files {
		migration, err := os.ReadFile(file)
		require.NoError(t, err)
		up, _, _ := strings.Cut(string(migration), "-- +goose Down")
		_, err = db.Exec(up)
		require.NoError(t, err, file)
	}
}

func skipIfDumpIsOlderThanServer(t *testing.T, admin *sql.DB) {
	t.Helper()
	var serverNum int
	require.NoError(t, admin.QueryRow(`SHOW server_version_num`).Scan(&serverNum))
	out, err := exec.Command("pg_dump", "--version").Output()
	require.NoError(t, err)
	m := regexp.MustCompile(`(\d+)`).FindStringSubmatch(string(out))
	require.NotNil(t, m)
	client, _ := strconv.Atoi(m[1])
	if client < serverNum/10000 {
		t.Skipf("pg_dump %d is older than server %d", client, serverNum/10000)
	}
}

func (e *splitEnv) run(t *testing.T) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "../../deploy/backend/split-db.sh")
	cmd.Env = append(os.Environ(),
		"SOURCE_DATABASE_URL="+e.sourceURL, "AUTH_DATABASE_URL="+e.authURL, "ADS_DATABASE_URL="+e.adsURL,
		"SKIP_SERVICES=1", "BACKUP_DIR="+t.TempDir(), "LOCK_FILE="+filepath.Join(t.TempDir(), "lock"))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func count(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM public.`+table).Scan(&n))
	return n
}

func TestSplitDB_CopiesDataAndKeepsSequences(t *testing.T) {
	e := newSplitEnv(t, 14)

	out, err := e.run(t)
	require.NoError(t, err, out)

	for _, table := range []string{"user_account", "city", "session"} {
		require.Equal(t, count(t, e.source, table), count(t, e.auth, table), table)
	}
	for _, table := range []string{"ad", "ad_image", "review", "address", "orders", "order_item", "category", "price_history", "ad_status_event"} {
		require.Equal(t, count(t, e.source, table), count(t, e.ads, table), table)
	}
	require.Positive(t, count(t, e.ads, "orders"))
	if u, err := url.Parse(e.authURL); err == nil {
		if password, ok := u.User.Password(); ok && password != "" {
			require.NotContains(t, out, password)
		}
	}

	var maxAd, newAd int
	require.NoError(t, e.source.QueryRow(`SELECT max(id) FROM ad`).Scan(&maxAd))
	require.NoError(t, e.ads.QueryRow(`INSERT INTO ad (title, user_id) VALUES ('Новое', 1) RETURNING id`).Scan(&newAd))
	require.Equal(t, maxAd+1, newAd)

	// Пустая таблица: следующий ID — 1, а не ошибка setval.
	var promotionID int
	require.NoError(t, e.ads.QueryRow(`INSERT INTO promotion (ad_id, end_date, promotion_level) VALUES ($1, now() + interval '1 day', 1) RETURNING prom_id`, newAd).Scan(&promotionID))
	require.Equal(t, 1, promotionID)

	// Триггеры не сработали при копировании: история цен и статусов осталась исходной.
	require.Equal(t, count(t, e.source, "price_history"), count(t, e.ads, "price_history"))
}

func TestSplitDB_RefusesFilledTargets(t *testing.T) {
	e := newSplitEnv(t, 14)
	out, err := e.run(t)
	require.NoError(t, err, out)
	before := count(t, e.ads, "ad")

	out, err = e.run(t)

	require.Error(t, err)
	require.Contains(t, out, "already contains data")
	require.Equal(t, before, count(t, e.ads, "ad"))
}

func TestSplitDB_RefusesWrongSourceVersion(t *testing.T) {
	e := newSplitEnv(t, 13)

	out, err := e.run(t)

	require.Error(t, err)
	require.Contains(t, out, "expected 14")
	var tables int
	require.NoError(t, e.auth.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'`).Scan(&tables))
	require.Zero(t, tables)
}
