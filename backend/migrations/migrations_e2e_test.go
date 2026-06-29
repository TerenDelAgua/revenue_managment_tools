// Migration E2E tests — apply all migrations in order to an embedded
// Postgres 16 and verify the resulting schema. Designed to run only when
// DATABASE_URL is NOT set (so it doesn't fight with the real DB).
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

func startEPG(t *testing.T, port uint32) string {
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

	return "postgres://teren:teren123@127.0.0.1:" + strconv.FormatUint(uint64(port), 10) + "/teren_hotels?sslmode=disable"
}

func assertInvoicingV12Shape(t *testing.T, db *sql.DB) {
	t.Helper()
	type check struct{ table, column string }
	required := []check{
		{"schema_migrations", "filename"},
		{"invoices", "status"},
		{"invoices", "needs_review"},
		{"invoices", "pdf_url"},
		{"invoices", "notes"},
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
	for _, table := range []string{"invoice_sequences", "invoice_line_items", "payments", "refund_batches", "idempotency_keys"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables
		                       WHERE table_name=$1`, table).Scan(&n); err != nil {
			t.Fatalf("table check %s: %v", table, err)
		}
		if n == 0 {
			t.Errorf("expected table %s to exist", table)
		}
	}
}

func applyFile(t *testing.T, db *sql.DB, name string) error {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	_, err = db.Exec(string(b))
	return err
}

// bootstrapLegacyCatalog creates the schema_migrations table without the
// `filename` column to mirror the legacy prod DB shape (pre-010 deployment).
func bootstrapLegacyCatalog(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		t.Fatalf("create legacy catalog: %v", err)
	}
}

// bootstrapGreenfieldCatalog creates the catalog in its latest shape.
func bootstrapGreenfieldCatalog(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		filename TEXT,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`); err != nil {
		t.Fatalf("create catalog: %v", err)
	}
}

// TestMigrationsApplyGreenfield: empty DB with the latest-shape runner
// catalog. All migrations must apply cleanly and produce the v1.2 schema.
func TestMigrationsApplyGreenfield(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	connStr := startEPG(t, 54329)
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
	bootstrapGreenfieldCatalog(t, db)

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

	assertInvoicingV12Shape(t, db)
}

// TestMigrationsApplyLegacyCatalog: legacy catalog without `filename`.
// Mirrors the exact state of the production database at the time the 008
// failure was reported. Migrations must self-heal and end at v1.2 shape.
func TestMigrationsApplyLegacyCatalog(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	connStr := startEPG(t, 54330)
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
	bootstrapLegacyCatalog(t, db)

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

	var colCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
	                       WHERE table_name='schema_migrations' AND column_name='filename'`).Scan(&colCount); err != nil {
		t.Fatalf("filename column check: %v", err)
	}
	if colCount == 0 {
		t.Fatal("expected a migration to add the `filename` column to schema_migrations")
	}
	assertInvoicingV12Shape(t, db)
}

// TestMigrationsApplyLegacyInvoicingTable: simulate a database where the
// legacy MVP `invoices` table exists with the vestigial schema and the
// 012 reconciliation has not yet run. Mirrors the production state at the
// moment 008 failed.
func TestMigrationsApplyLegacyInvoicingTable(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	connStr := startEPG(t, 54331)
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
	bootstrapLegacyCatalog(t, db)

	// Mark version 6 as already applied (the way production does), so the
	// runner skips 006 and we end up in the same state as the real prod DB.
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at)
	                       VALUES (6, NOW()) ON CONFLICT (version) DO NOTHING`); err != nil {
		t.Fatalf("seed catalog: %v", err)
	}

	if err := applyFile(t, db, "001_initial_schema.sql"); err != nil {
		t.Fatalf("apply 001 (legacy): %v", err)
	}

	var hasPaymentStatus int
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
	                       WHERE table_name='invoices' AND column_name='payment_status'`).Scan(&hasPaymentStatus); err != nil {
		t.Fatalf("legacy check: %v", err)
	}
	if hasPaymentStatus == 0 {
		t.Fatal("expected legacy `payment_status` column on invoices")
	}

	// Apply every other migration. Some may fail on the legacy shape —
	// 012 reconciliation must bring the DB to v1.2 regardless.
	list := readMigrations(t, ".")
	for _, m := range list {
		if m.name == "001_initial_schema.sql" {
			continue
		}
		b, err := os.ReadFile(m.path)
		if err != nil {
			t.Fatalf("read %s: %v", m.name, err)
		}
		if _, err := db.Exec(string(b)); err != nil {
			t.Logf("Legacy-aware: %03d %s failed (%v) — expected if shape mismatch", m.ver, m.name, err)
		}
	}

	assertInvoicingV12Shape(t, db)
}

// TestMigrationsApplyIdempotentlyPerFile: every migration applied twice in
// isolation must not raise. Catches regressions where a single file loses
// its idempotent guards (the "001 to 012 batch idempotency" check above
// cannot distinguish between two bad files cancelling out).
func TestMigrationsApplyIdempotentlyPerFile(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	connStr := startEPG(t, 54333)
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
	bootstrapGreenfieldCatalog(t, db)

	list := readMigrations(t, ".")
	// First pass — bring the DB to v1.2.
	for _, m := range list {
		if err := applyFile(t, db, m.name); err != nil {
			t.Fatalf("first pass apply %s: %v", m.name, err)
		}
	}

	// Second pass — every migration must be a no-op.
	for _, m := range list {
		if err := applyFile(t, db, m.name); err != nil {
			t.Errorf("second pass apply %s: %v", m.name, err)
		}
	}

	assertInvoicingV12Shape(t, db)
}

// TestMigrationsApplyIdempotentlyBatch: every migration applied twice in
// order must not raise and must leave the schema identical.
func TestMigrationsApplyIdempotentlyBatch(t *testing.T) {
	if os.Getenv("DATABASE_URL") != "" {
		t.Skip("DATABASE_URL set — skipping embedded-postgres validation.")
	}

	connStr := startEPG(t, 54332)
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
	bootstrapGreenfieldCatalog(t, db)

	apply := func() {
		list := readMigrations(t, ".")
		for _, m := range list {
			b, err := os.ReadFile(m.path)
			if err != nil {
				t.Fatalf("read %s: %v", m.name, err)
			}
			if _, err := db.Exec(string(b)); err != nil {
				t.Fatalf("re-apply %s: %v", m.name, err)
			}
		}
	}

	apply()
	assertInvoicingV12Shape(t, db)
	apply()
	assertInvoicingV12Shape(t, db)
}