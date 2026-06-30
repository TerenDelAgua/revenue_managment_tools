package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime configuration. New fields are added as
// modules grow; nothing else outside this package should call os.Getenv.
type Config struct {
	Port        string
	DatabaseURL string

	// Object storage for invoice PDFs (Cloudflare R2 or local FS).
	// If R2Endpoint is empty, the PDF generator falls back to a local
	// FS store at LocalPDFDir. See pkg/pdfstore.
	R2Endpoint  string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
	R2PublicURL string // base URL for the R2 bucket (used to build pdf_url)

	LocalPDFDir string // used when R2Endpoint is empty

	// PublicBaseURL is the externally-reachable origin where this API
	// is served (scheme + host, no trailing slash). It is used to build
	// absolute URLs to local PDFs when no R2 bucket is configured.
	//
	// Resolution order (highest priority first):
	//   1. PUBLIC_BASE_URL env var (explicit override).
	//   2. RAILWAY_PUBLIC_DOMAIN — auto-set by Railway for every
	//      deployed service. We prepend "https://" so it matches the
	//      format of (1) without manual config.
	//   3. "" (empty) — the PDF store falls back to PDFBaseURL, which
	//      itself defaults to http://localhost:8080/api/v1/pdfs.
	PublicBaseURL string

	// PDFBaseURL is the public HTTP base from which local PDFs are
	// served. The dev PDF handler exposes them at
	// {PDFBaseURL}/{object-key}. In production (R2), this field is
	// ignored — R2 has its own CDN.
	//
	// Defaults to {PublicBaseURL}/api/v1/pdfs when PublicBaseURL is
	// resolved, falling back to http://localhost:8080/api/v1/pdfs.
	PDFBaseURL string
}

// Load reads .env (if present) and returns a Config with sensible
// defaults for local development.
func Load() (*Config, error) {
	_ = godotenv.Overload()

	publicBase := resolvePublicBaseURL()

	pdfBase := strings.TrimRight(getEnv("PDF_BASE_URL", ""), "/")
	if pdfBase == "" {
		if publicBase != "" {
			pdfBase = publicBase + "/api/v1/pdfs"
		} else {
			pdfBase = "http://localhost:8080/api/v1/pdfs"
		}
	}

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable"),

		R2Endpoint:  getEnv("R2_ENDPOINT", ""),
		R2AccessKey: getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:    getEnv("R2_BUCKET", ""),
		R2PublicURL: strings.TrimRight(getEnv("R2_PUBLIC_URL", ""), "/"),

		LocalPDFDir:   getEnv("LOCAL_PDF_DIR", "./tmp/pdfs"),
		PublicBaseURL: publicBase,
		PDFBaseURL:    pdfBase,
	}, nil
}

// resolvePublicBaseURL picks the externally-reachable origin of the
// API from the environment. See PublicBaseURL for precedence.
func resolvePublicBaseURL() string {
	if v := strings.TrimSpace(getEnv("PUBLIC_BASE_URL", "")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(getEnv("RAILWAY_PUBLIC_DOMAIN", "")); v != "" {
		return "https://" + strings.TrimRight(v, "/")
	}
	return ""
}

// UseR2 reports whether R2 credentials are configured. The PDF store
// factory uses this to pick R2 vs local FS at startup.
func (c *Config) UseR2() bool {
	return c.R2Endpoint != "" && c.R2AccessKey != "" && c.R2SecretKey != "" && c.R2Bucket != ""
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
