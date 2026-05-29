package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Migration struct {
	Version  int
	Filename string
	Path     string
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	migrationsDir := "migrations"
	if len(os.Args) > 2 && os.Args[1] == "-dir" {
		migrationsDir = os.Args[2]
	}

	log.Printf("Connecting to database...")
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

	// Create schema_migrations table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		applied_at TIMESTAMPTZ DEFAULT NOW()
	);`)
	if err != nil {
		log.Fatalf("Failed to create schema_migrations table: %v", err)
	}

	// Read migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Fatalf("Failed to read migrations directory: %v", err)
	}

	var migrations []Migration
	versionRegex := regexp.MustCompile(`^(\d+)[_-]`)

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if !strings.HasSuffix(name, ".sql") || strings.HasSuffix(name, ".down.sql") {
			continue
		}

		matches := versionRegex.FindStringSubmatch(name)
		if len(matches) < 2 {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			log.Printf("Warning: Failed to parse version from %s: %v", name, err)
			continue
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Filename: name,
			Path:     filepath.Join(migrationsDir, name),
		})
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	log.Printf("Found %d up migration(s) in %s", len(migrations), migrationsDir)

	for _, m := range migrations {
		var exists bool
		err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", m.Version).Scan(&exists)
		if err != nil {
			log.Fatalf("Failed to check if migration %d is applied: %v", m.Version, err)
		}

		if exists {
			log.Printf("Migration %03d (%s) already applied. Skipping.", m.Version, m.Filename)
			continue
		}

		log.Printf("Applying migration %03d (%s)...", m.Version, m.Filename)
		content, err := os.ReadFile(m.Path)
		if err != nil {
			log.Fatalf("Failed to read migration file %s: %v", m.Filename, err)
		}

		tx, err := db.Begin()
		if err != nil {
			log.Fatalf("Failed to start transaction: %v", err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to execute migration %s: %v", m.Filename, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", m.Version); err != nil {
			tx.Rollback()
			log.Fatalf("Failed to record migration version %d: %v", m.Version, err)
		}

		if err := tx.Commit(); err != nil {
			log.Fatalf("Failed to commit transaction for migration %s: %v", m.Filename, err)
		}

		log.Printf("Successfully applied migration %03d.", m.Version)
	}

	log.Println("Database is up to date.")
}
