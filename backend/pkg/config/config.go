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

	// PDFBaseURL is the public HTTP base from which local PDFs are
	// served. The dev PDF handler exposes them at
	// {PDFBaseURL}/{object-key}. Default points at the local API on 8080.
	// In production (R2), this field is ignored — R2 has its own CDN.
	PDFBaseURL string
}

// Load reads .env (if present) and returns a Config with sensible
// defaults for local development.
func Load() (*Config, error) {
	_ = godotenv.Overload()

	return &Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://teren:teren123@localhost:5432/teren_hotels?sslmode=disable"),

		R2Endpoint:  getEnv("R2_ENDPOINT", ""),
		R2AccessKey: getEnv("R2_ACCESS_KEY_ID", ""),
		R2SecretKey: getEnv("R2_SECRET_ACCESS_KEY", ""),
		R2Bucket:    getEnv("R2_BUCKET", ""),
		R2PublicURL: strings.TrimRight(getEnv("R2_PUBLIC_URL", ""), "/"),

		LocalPDFDir: getEnv("LOCAL_PDF_DIR", "./tmp/pdfs"),
		PDFBaseURL:  strings.TrimRight(getEnv("PDF_BASE_URL", "http://localhost:8080/api/v1/pdfs"), "/"),
	}, nil
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
