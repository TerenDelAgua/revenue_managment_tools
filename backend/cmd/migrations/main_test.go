package main

import (
	"path/filepath"
	"testing"
)

func TestParseRequirements_NoHeader(t *testing.T) {
	got, err := parseRequirements(filepath.Join("testdata", "no_requires.sql"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 requirements, got %d: %+v", len(got), got)
	}
}

func TestParseRequirements_SingleColumn(t *testing.T) {
	got, err := parseRequirements(filepath.Join("testdata", "single_require.sql"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(got))
	}
	if got[0].Table != "invoices" || got[0].Column != "status" {
		t.Errorf("expected invoices.status, got %+v", got[0])
	}
}

func TestParseRequirements_MultipleColumns(t *testing.T) {
	got, err := parseRequirements(filepath.Join("testdata", "multi_requires.sql"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 requirements, got %d", len(got))
	}
	if got[0].Table != "schema_migrations" || got[0].Column != "filename" {
		t.Errorf("expected schema_migrations.filename, got %+v", got[0])
	}
	if got[1].Table != "invoices" || got[1].Column != "status" {
		t.Errorf("expected invoices.status, got %+v", got[1])
	}
}

func TestParseRequirements_Malformed(t *testing.T) {
	_, err := parseRequirements(filepath.Join("testdata", "malformed_require.sql"))
	if err == nil {
		t.Fatal("expected error for malformed requirement, got nil")
	}
}

func TestDiscoverMigrations_DropsDownSuffix(t *testing.T) {
	files, err := discoverMigrations(filepath.Join("testdata", "mixed"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 up migration, got %d", len(files))
	}
	if files[0].Filename != "005_keep.sql" {
		t.Errorf("expected 005_keep.sql, got %s", files[0].Filename)
	}
}

func TestAssertNoVersionCollisions_OK(t *testing.T) {
	migs := []Migration{
		{Version: 1, Filename: "001_init.sql"},
		{Version: 2, Filename: "002_more.sql"},
	}
	if err := assertNoVersionCollisions(nil, migs); err != nil {
		t.Errorf("expected no collision, got %v", err)
	}
}

func TestAssertNoVersionCollisions_Duplicate(t *testing.T) {
	migs := []Migration{
		{Version: 6, Filename: "006_add_notes.sql"},
		{Version: 6, Filename: "006_invoicing_schema.up.sql"},
	}
	err := assertNoVersionCollisions(nil, migs)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
}
