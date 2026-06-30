// Package pdfstore stores generated PDF bytes durably and returns a
// publicly-accessible URL. Two backends are supported:
//
//   - R2 (production): Cloudflare R2 via the S3-compatible minio-go client.
//   - LocalStore (dev/test): writes to a local directory and returns a
//     file:// URL. No external dependencies.
//
// The backend is selected by the factory (NewStore) based on whether
// the config has R2 credentials. See config.Config.UseR2.
package pdfstore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/terendelagua/teren-hotels-backend/pkg/config"
)

// Store is the common interface for storing generated PDFs.
type Store interface {
	// Put stores the PDF under the given key (e.g. "invoices/INV-2026-0001.pdf")
	// and returns the publicly-accessible URL.
	Put(ctx context.Context, key string, data []byte) (string, error)
}

// NewStore picks the right backend based on the config. If R2 credentials
// are present, returns a R2Store. Otherwise, returns a LocalStore.
func NewStore(cfg *config.Config) (Store, error) {
	if cfg.UseR2() {
		return NewR2Store(cfg)
	}
	return NewLocalStore(cfg)
}

// =============================================================================
// Local FS (dev / test)
// =============================================================================

type LocalStore struct {
	dir       string
	publicURL string
}

// NewLocalStore is the public constructor (exported so it can be
// tested directly and reused from the test suite).
func NewLocalStore(cfg *config.Config) (*LocalStore, error) {
	if err := os.MkdirAll(cfg.LocalPDFDir, 0o755); err != nil {
		return nil, fmt.Errorf("create local pdf dir: %w", err)
	}
	// publicURL is the HTTP base where the API serves local PDFs from.
	// We never return file:// URLs — browsers block them from JS and
	// the SPA needs to fetch the PDF as a normal same-origin resource.
	//
	// Precedence:
	//   1. cfg.PDFBaseURL — explicit override via PDF_BASE_URL.
	//   2. cfg.PublicBaseURL + "/api/v1/pdfs" — auto-detected from
	//      PUBLIC_BASE_URL or RAILWAY_PUBLIC_DOMAIN by config.Load.
	//   3. http://localhost:8080/api/v1/pdfs — local dev fallback.
	publicURL := cfg.PDFBaseURL
	if publicURL == "" {
		if cfg.PublicBaseURL != "" {
			publicURL = cfg.PublicBaseURL + "/api/v1/pdfs"
		} else {
			publicURL = "http://localhost:8080/api/v1/pdfs"
		}
	}
	return &LocalStore{dir: cfg.LocalPDFDir, publicURL: publicURL}, nil
}

func (s *LocalStore) Put(_ context.Context, key string, data []byte) (string, error) {
	// key is "invoices/INV-2026-0001.pdf" — split into dir + filename.
	dir, file := filepath.Split(key)
	fullDir := filepath.Join(s.dir, dir)
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", fmt.Errorf("create subdir: %w", err)
	}
	full := filepath.Join(fullDir, file)
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return "", fmt.Errorf("write pdf: %w", err)
	}
	// Return an HTTP URL the SPA can fetch without security warnings.
	// Always forward-slash — Go's URL parser doesn't honour OS-specific
	// separators here and we want a stable, portable URL.
	return s.publicURL + "/" + filepath.ToSlash(key), nil
}

// =============================================================================
// Cloudflare R2 (production) — S3-compatible via minio-go
// =============================================================================

type R2Store struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

// NewR2Store is the public constructor for the R2 (S3-compatible) backend.
func NewR2Store(cfg *config.Config) (*R2Store, error) {
	client, err := minio.New(cfg.R2Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.R2AccessKey, cfg.R2SecretKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, fmt.Errorf("init r2 client: %w", err)
	}
	return &R2Store{
		client:    client,
		bucket:    cfg.R2Bucket,
		publicURL: cfg.R2PublicURL,
	}, nil
}

func (s *R2Store) Put(ctx context.Context, key string, data []byte) (string, error) {
	// Minio expects a Reader + size.
	_, err := s.client.PutObject(ctx, s.bucket, key,
		ioReader(data), int64(len(data)), minio.PutObjectOptions{
			ContentType: "application/pdf",
		})
	if err != nil {
		return "", fmt.Errorf("r2 put: %w", err)
	}
	// Public URL: prefer the configured R2 public base URL (e.g.
	// https://pub-xxxx.r2.dev), fall back to the S3 endpoint.
	if s.publicURL != "" {
		return s.publicURL + "/" + key, nil
	}
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s/%s",
		s.bucket, s.bucket, key), nil
}

// ioReader is a tiny adapter to avoid pulling io.NewSectionReader or
// bytes.NewReader at the call site. Slices of bytes implement io.Reader
// already, but minio's PutObject takes io.Reader for the body.
func ioReader(b []byte) io.Reader { return ioReaderHelper(b) }

type ioReaderHelper []byte

func (r ioReaderHelper) Read(p []byte) (int, error) {
	n := copy(p, r)
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// =============================================================================
// Helper: build a stable object key
// =============================================================================

// ObjectKey returns the storage key for an invoice PDF. Format:
//
//	{property_id}/invoices/{invoice_number}.pdf
func ObjectKey(propertyID uuid.UUID, invoiceNumber string) string {
	return fmt.Sprintf("%s/invoices/%s.pdf", propertyID, invoiceNumber)
}
