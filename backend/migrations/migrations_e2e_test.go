// Migration E2E test — applies all migrations in order to an embedded
// Postgres 16 and verifies the resulting schema. Designed to run only
// when DATABASE_URL is NOT set (so it doesn't fight with the real DB).
// Run with: go test -tags migration_e2e ./migrations/...
//go:build migration_e2e

package migrations

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	_ "github.com/lib/pq"
)

type mig struct {
	ver  int
	name string
	path string
}

func readMigrations(t *testing.T, dir string) []mig {
	t.Helper()
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	re := regexp.MustCompile(`^(\d+)[_-]`)
	var list []mig
	for _, f := range files {
		n := f.Name()
		if !strings.HasSuffix(n, ".sql") || strings.HasSuffix(n, ".down.sql") || strings.HasSuffix(n, ".bak") {
			continue
		}
		m := re.FindStringSubmatch(n)
		if len(m) < 2 {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		list = append(list, mig{ver: v, name: n, path: filepath.Join(dir, n)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ver < list[j].ver })
	return list
}

func startEPG(t *testing.T, port uint32) (*embeddedpostgres.EmbeddedPostgres, string) {
	t.Helper()
	dataDir := filepath.Join(os.TempDir(), "teren-epg-mig-test")
	_ = os.RemoveAll(dataDir)
	t.Cleanup(func() { os.RemoveAll(dataDir) })

	cfg := embeddedpostgres.DefaultConfig().
		Username("teren").
		Password("teren123").
		Database("teren_hotels").
		Port(port).
		Version(embeddedpostgres.V16).
		Locale("en_US.UTF-8")
	epg := embeddedpostgres.NewDatabase(cfg)
	if err := epg.Start(); err != nil {
		t.Fatalf("start epg: %v", err)
	}
	t.Cleanup(func() { epg.Stop() })

	return epg, "postgres://teren:teren123@127.0.0.1:" + strconv.FormatUint(uint64(port), 10) + "/teren_hotels?sslmode=disable"
}

// assertRequiredSchema verifies the post-migration shape expected by
// invoicing v1.2 (R-07, R-08) and the runner catalog (filename).
func assertRequiredSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	type check struct{ table, column string }
	required := []check{
		{"schema_migrations", "filename"},
		{"invoices", "status"},
		{"invoices", "needs_review"},
		{"payments", "invalidated_at"},
		{"payments", "invalidated_by"},
		{"payments", "invalidated_reason"},
	}
	for _, c := range required {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		                       WHERE table_name=$1 AND column_name=$2`,
			c.table, c.column).Scan(&n); err != nil {
			t.Fatalf("check %s.%s: %v", c.table, c.column, err)
		}
		if n == 0 {
			t.Errorf("expected column %s.%s to exist", c.table, c.column)
		}
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
	                       WHERE table_name='refund_batches'`).Scan(&n); err != nil {
		t.Fatalf("refund_batches check: %v", err)
	}
	if n == 0 {
		t.Error("expected refund_batches table to exist")
	}
}

// TestMigrationsApplyClean: greenfield — fresh DB, latest-shape runner catalog.
func TestMigrationsApplyClean(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	_, connStr := startEPG(t, 54329)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		t.Fatalf("create ext: %v", err)
	}

	// Latest-shape runner catalog (includes `filename`).
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		filename TEXT,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		t.Fatalf("create catalog: %v", err)
	}

	list := readMigrations(t, ".")
	for _, m := range list {
		b, err := os.ReadFile(m.path)
		if err != nil {
			t.Fatalf("read %s: %v", m.name, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("apply %s: %v", m.name, err)
		}
		t.Logf("Applied %03d %s", m.ver, m.name)
	}

	assertRequiredSchema(t, db)

	// Idempotency: re-apply the 010 migration.
	reapply := "010_invoice_refunded_status_fix.up.sql"
	b, err := os.ReadFile(reapply)
	if err != nil {
		t.Fatalf("read %s for reapply: %v", reapply, err)
	}
	if _, err := db.Exec(string(b)); err != nil {
		t.Fatalf("idempotency: re-applying %s must not fail, got: %v", reapply, err)
	}
	assertRequiredSchema(t, db)
}

// TestMigrationsApplyLegacyCatalog: production-like scenario where the runner
// was bootstrapped with an older `schema_migrations(version, applied_at)`
// shape (no `filename` column). The 010 migration itself must add the column
// so the runner can subsequently insert filenames.
func TestMigrationsApplyLegacyCatalog(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	_, connStr := startEPG(t, 54330)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if _, err := db.Exec(`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`); err != nil {
		t.Fatalf("create ext: %v", err)
	}

	// Legacy catalog: no `filename` column. The runner's own code only
	// does `CREATE TABLE IF NOT EXISTS`, so it would never add `filename`
	// by itself — that must come from a migration.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		t.Fatalf("create legacy catalog: %v", err)
	}

	list := readMigrations(t, ".")
	for _, m := range list {
		b, err := os.ReadFile(m.path)
		if err != nil {
			t.Fatalf("read %s: %v", m.name, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Fatalf("apply %s: %v", m.name, err)
		}
		t.Logf("Applied %03d %s", m.ver, m.name)
	}

	// Confirm 010 alone was responsible for adding the filename column —
	// it should be queryable now and accept subsequent INSERTs that set it.
	var colCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
	                       WHERE table_name='schema_migrations' AND column_name='filename'`).Scan(&colCount); err != nil {
		t.Fatalf("filename column check: %v", err)
	}
	if colCount == 0 {
		t.Fatal("expected 010 to add the `filename` column to schema_migrations")
	}

	// Simulate the runner writing a row that uses `filename` to make sure
	// the column type is usable (TEXT, NULLABLE).
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, filename)
	                       VALUES ($1, $2) ON CONFLICT (version) DO UPDATE SET filename = EXCLUDED.filename`,
		int64(123), "manual-fix-marker.sql"); err != nil {
		t.Fatalf("insert with filename: %v", err)
	}

	assertRequiredSchema(t, db)
}
