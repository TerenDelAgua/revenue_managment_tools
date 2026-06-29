package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
)

// Migration is a single up migration file discovered in the migrations
// directory. The runner sorts by Version (numeric) and applies them in
// order against the catalog table schema_migrations.
type Migration struct {
	Version  int
	Filename string
	Path     string
	Requires []MigrationRequirement
}

// MigrationRequirement declares an external precondition that must be
// satisfied before the migration is applied. Sourced from the
// `-- requires:` header line in the SQL file:
//
//	-- requires: invoices.status
//	-- requires: schema_migrations.filename
//
// Format: comma-separated `column` references where `column` is either a
// `<table>.<column>` pair or `schema_migrations.filename` etc.
type MigrationRequirement struct {
	Table  string
	Column string
}

func main() {
	_ = godotenv.Overload()

	var (
		dryRun       = flag.Bool("dry-run", false, "Apply pending migrations inside a transaction and rollback. Exit 0 if all would succeed, 1 otherwise.")
		migrationsDir = flag.String("dir", "migrations", "Directory containing up migration files.")
	)
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	log.Printf("Connecting to database (dry-run=%v)...", *dryRun)
	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database successfully.")

	if err := bootstrapCatalog(db); err != nil {
		log.Fatalf("Failed to bootstrap runner catalog: %v", err)
	}

	migrations, err := discoverMigrations(*migrationsDir)
	if err != nil {
		log.Fatalf("Failed to discover migrations: %v", err)
	}
	log.Printf("Found %d up migration(s) in %s", len(migrations), *migrationsDir)

	if err := assertNoVersionCollisions(db, migrations); err != nil {
		log.Fatalf("Migration catalog conflict (filename/version mismatch): %v", err)
	}

	pending, err := filterPending(db, migrations)
	if err != nil {
		log.Fatalf("Failed to read pending migrations: %v", err)
	}
	log.Printf("Pending migrations: %d", len(pending))

	for _, m := range pending {
		warnMissingRequires(m)

		if err := assertRequirements(db, m); err != nil {
			log.Fatalf("Pre-flight failed for %s: %v", m.Filename, err)
		}

		content, err := os.ReadFile(m.Path)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", m.Filename, err)
		}

		if *dryRun {
			log.Printf("[dry-run] Would apply migration %03d (%s)", m.Version, m.Filename)
			if err := runInRollbackOnly(db, string(content), m); err != nil {
				log.Fatalf("[dry-run] FAIL %03d (%s): %v", m.Version, m.Filename, err)
			}
			continue
		}

		log.Printf("Applying migration %03d (%s)...", m.Version, m.Filename)
		if err := applyMigration(db, string(content), m); err != nil {
			log.Fatalf("Failed to execute %s: %v", m.Filename, err)
		}
		log.Printf("Successfully applied migration %03d.", m.Version)
	}

	log.Println("Database is up to date.")
}

// bootstrapCatalog creates the runner's catalog table if it does not
// exist. Schema evolution of this table is handled by migration files
// (e.g. 010 added `filename`), not by inline ALTER statements, so this
// function only ensures the table itself exists in its latest known shape.
func bootstrapCatalog(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		filename TEXT,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	)`)
	return err
}

// discoverMigrations reads `dir` and returns all up migrations sorted by
// numeric version. Files starting with a leading numeric prefix followed
// by `_` or `-` and ending in `.sql` are considered. `.down.sql` files
// are skipped.
func discoverMigrations(dir string) ([]Migration, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	re := regexp.MustCompile(`^(\d+)[_-]`)
	var list []Migration
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".down.sql") {
			continue
		}
		m := re.FindStringSubmatch(name)
		if len(m) < 2 {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			log.Printf("Warning: failed to parse version from %s: %v", name, err)
			continue
		}
		path := filepath.Join(dir, name)
		reqs, err := parseRequirements(path)
		if err != nil {
			return nil, fmt.Errorf("parse requires in %s: %w", name, err)
		}
		list = append(list, Migration{
			Version:  v,
			Filename: name,
			Path:     path,
			Requires: reqs,
		})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

// requiresHeaderRe matches a top-of-file header line that declares
// preconditions for the migration. Example:
//
//	-- requires: schema_migrations.filename, invoices.status
//
// Captures everything after `requires:` until end of line. The trailing
// `\s*` is optional so a header with no trailing whitespace still parses.
var requiresHeaderRe = regexp.MustCompile(`(?m)^--\s*requires:\s*([^\n]+?)\s*$`)

// parseRequirements reads the first `headerScanLines` lines of `path`
// looking for a `-- requires:` header line and parses the referenced
// table/column pairs. Scanning is restricted to the file header so a
// `-- requires:` comment buried in the middle of a SQL script cannot
// accidentally be interpreted as a precondition. Returns an empty slice
// if no header is declared.
func parseRequirements(path string) ([]MigrationRequirement, error) {
	const headerScanLines = 20
	const headerScanBytes = 64 * 1024

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, headerScanBytes)
	n, _ := f.Read(buf)
	header := string(buf[:n])

	scanned := header
	if idx := strings.Index(header, "\n"); idx >= 0 {
		lines := strings.SplitN(header, "\n", headerScanLines+1)
		if len(lines) > headerScanLines {
			scanned = strings.Join(lines[:headerScanLines], "\n")
		}
	}

	match := requiresHeaderRe.FindStringSubmatch(scanned)
	if len(match) < 2 {
		return nil, nil
	}
	parts := strings.Split(match[1], ",")
	var out []MigrationRequirement
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		bits := strings.SplitN(p, ".", 2)
		if len(bits) != 2 {
			return nil, fmt.Errorf("malformed requirement %q (expected table.column)", p)
		}
		out = append(out, MigrationRequirement{Table: bits[0], Column: bits[1]})
	}
	return out, nil
}

// assertNoVersionCollisions checks whether two migration files share the
// same Version but report different Filenames. This catches the
// "006_add_notes.sql + 006_invoicing_schema.up.sql" class of bug before
// any DDL is applied.
//
// `db` may be nil when only the in-memory collision check is desired
// (e.g. unit tests). When non-nil, a row in `schema_migrations` for the
// same version with a different filename is also reported as a collision.
func assertNoVersionCollisions(db *sql.DB, migrations []Migration) error {
	byVersion := make(map[int][]string)
	for _, m := range migrations {
		byVersion[m.Version] = append(byVersion[m.Version], m.Filename)
	}
	for v, files := range byVersion {
		if len(files) > 1 {
			return fmt.Errorf("version %03d is declared by more than one file: %s", v, strings.Join(files, ", "))
		}
	}
	if db == nil {
		return nil
	}
	for _, m := range migrations {
		var recorded sql.NullString
		err := db.QueryRow(`SELECT filename FROM schema_migrations WHERE version = $1`, m.Version).Scan(&recorded)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("read catalog for version %d: %w", m.Version, err)
		}
		if recorded.Valid && recorded.String != "" && recorded.String != m.Filename {
			return fmt.Errorf("version %03d is recorded with filename %q in the catalog, but the migrations directory now has %q", m.Version, recorded.String, m.Filename)
		}
	}
	return nil
}

// filterPending returns the subset of `migrations` whose version has not
// yet been recorded in `schema_migrations`.
func filterPending(db *sql.DB, migrations []Migration) ([]Migration, error) {
	var pending []Migration
	for _, m := range migrations {
		var exists bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)`, m.Version).Scan(&exists)
		if err != nil {
			return nil, fmt.Errorf("check applied for v%d: %w", m.Version, err)
		}
		if exists {
			log.Printf("Migration %03d (%s) already applied. Skipping.", m.Version, m.Filename)
			continue
		}
		pending = append(pending, m)
	}
	return pending, nil
}

// warnMissingRequires logs a non-fatal warning when a migration lacks a
// `-- requires:` header. This is informational only: the runner does not
// block on it, but the policy in AGENTS.md (single source of truth for
// prerequisites) is that every new migration should declare one. CI runs
// `go test ./cmd/migrations/... -run Requires` to enforce this when needed.
func warnMissingRequires(m Migration) {
	if len(m.Requires) == 0 {
		log.Printf("WARNING: %s has no `-- requires:` header. Add one in the file header (first 20 lines) listing every table.column this migration mutates.", m.Filename)
	}
}

// assertRequirements verifies that every `-- requires:` declaration on
// the migration resolves against the current catalog. Each requirement
// is checked by querying information_schema for the table/column pair.
func assertRequirements(db *sql.DB, m Migration) error {
	if len(m.Requires) == 0 {
		return nil
	}
	for _, r := range m.Requires {
		var n int
		err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns
		                    WHERE table_name=$1 AND column_name=$2`,
			r.Table, r.Column).Scan(&n)
		if err != nil {
			return fmt.Errorf("require %s.%s: %w", r.Table, r.Column, err)
		}
		if n == 0 {
			return fmt.Errorf("required column %s.%s is missing — apply an earlier migration or run migrations/reconciliations first", r.Table, r.Column)
		}
	}
	return nil
}

// runInRollbackOnly executes the SQL inside a transaction and rolls back
// unconditionally. Used by --dry-run to validate the SQL without
// committing anything.
func runInRollbackOnly(db *sql.DB, content string, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(content); err != nil {
		return err
	}
	return nil
}

// applyMigration executes the SQL inside a transaction and records the
// migration in schema_migrations atomically. Catalog schema is now part
// of the same transaction, so a failed migration leaves no orphan rows.
func applyMigration(db *sql.DB, content string, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		// Defensive: if anything failed before Commit, ensure rollback.
		_ = tx.Rollback()
	}()

	if _, err := tx.Exec(content); err != nil {
		return fmt.Errorf("execute SQL: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, filename) VALUES ($1, $2)
	                       ON CONFLICT (version) DO UPDATE SET filename = EXCLUDED.filename`,
		m.Version, m.Filename); err != nil {
		return fmt.Errorf("record migration in catalog: %w", err)
	}
	return tx.Commit()
}
