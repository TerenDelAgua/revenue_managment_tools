package main

import "testing"

// TestDetectProduction_ExplicitEnvFlag covers the legacy path: the
// operator sets `-env=production` and the guard fires regardless of
// the URL.
func TestDetectProduction_ExplicitEnvFlag(t *testing.T) {
	cases := []struct {
		name    string
		envFlag string
		dbURL   string
		want    bool
	}{
		{"production flag with local URL", "production", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable", true},
		{"staging flag with local URL", "staging", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable", false},
		{"development flag with local URL", "development", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable", false},
		{"empty flag with local URL", "", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectProduction(tc.envFlag, tc.dbURL)
			if got != tc.want {
				t.Errorf("detectProduction(%q, %q) = %v, want %v", tc.envFlag, tc.dbURL, got, tc.want)
			}
		})
	}
}

// TestDetectProduction_URLMarkers covers the safety net added in the
// 2026-06-30 seed fix: a managed cloud URL must be flagged as
// production even when the operator forgets `-env=production`.
func TestDetectProduction_URLMarkers(t *testing.T) {
	cases := []struct {
		name   string
		dbURL  string
		marker string
	}{
		{"railway", "postgres://postgres:secret@containers-us-west-4.railway.app:5432/railway", "railway"},
		{"railway internal", "postgres://postgres:secret@postgres.railway.internal:5432/railway", "railway"},
		{"aws rds", "postgres://admin:secret@mydb.cluster-abc123.eu-west-1.rds.amazonaws.com:5432/teren", "amazonaws"},
		{"render", "postgres://user:secret@dpg-xyz.oregon.render.com:5432/teren", "render.com"},
		{"supabase", "postgres://postgres:secret@db.abcdefghijk.supabase.co:5432/postgres", "supabase.co"},
		{"neon", "postgres://user:secret@ep-cool-name-123456.eu-central-1.aws.neon.tech/neondb", "neon.tech"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectProduction("development", tc.dbURL)
			if !got {
				t.Errorf("detectProduction(development, %q) = false, want true (URL contains %q)", tc.dbURL, tc.marker)
			}
		})
	}
}

// TestDetectProduction_LocalURLsStayNonProduction makes sure the
// guard does not false-positive on the docker-compose database
// shipped with the dev environment.
func TestDetectProduction_LocalURLsStayNonProduction(t *testing.T) {
	cases := []string{
		"postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable",
		"postgres://teren:teren123@127.0.0.1:5432/teren_hotels?sslmode=disable",
		"postgres://user:pwd@db:5432/teren_hotels?sslmode=disable",
		"",
	}
	for _, dbURL := range cases {
		t.Run(dbURL, func(t *testing.T) {
			if detectProduction("development", dbURL) {
				t.Errorf("detectProduction(development, %q) = true, want false", dbURL)
			}
		})
	}
}

// TestDetectProduction_IsCaseInsensitive guards against developers
// pasting a URL with Railway spelled differently (RAILWAY, Railway.App …).
func TestDetectProduction_IsCaseInsensitive(t *testing.T) {
	cases := []string{
		"postgres://user:pwd@db.RAILWAY.APP:5432/railway",
		"postgres://user:pwd@db.Railway.App:5432/railway",
		"postgres://user:pwd@db.RAILWAY.internal:5432/railway",
	}
	for _, dbURL := range cases {
		t.Run(dbURL, func(t *testing.T) {
			if !detectProduction("development", dbURL) {
				t.Errorf("detectProduction(development, %q) = false, want true", dbURL)
			}
		})
	}
}
