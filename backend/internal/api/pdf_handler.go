package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

// PDFHandler streams invoice PDFs that live on the local filesystem.
// In production (R2), this handler is bypassed — the SPA fetches the
// public R2 URL directly.
//
// Security
//   - The object key MUST match the storage convention enforced by
//     pdfstore.ObjectKey: {uuid}/invoices/{anything.pdf}. Anything
//     else is rejected.
//   - The resolved path is checked to be under cfg.LocalPDFDir to
//     prevent traversal (../foo).
//   - Missing files return 404, not 500 (cleaner UX).
type PDFHandler struct {
	rootDir string
}

// NewPDFHandler builds the handler. rootDir is the absolute or
// process-relative LocalPDFDir from config.
func NewPDFHandler(rootDir string) *PDFHandler {
	return &PDFHandler{rootDir: rootDir}
}

// objectKeyPattern accepts {uuid}/invoices/{file.pdf}. The filename is
// loose so we keep room for future variants (refunds, tax receipts…)
// without touching this regex.
var objectKeyPattern = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}/invoices/[^/]+\.pdf$`,
)

// Serve streams the PDF for the requested object key.
//
// The route in main.go is `/pdfs/*` so chi will pass everything after
// `/pdfs/` as a single path segment. We rebuild the key from the
// request URL to keep the path canonical.
func (h *PDFHandler) Serve(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "*")
	if key == "" || !objectKeyPattern.MatchString(key) {
		http.Error(w, "invalid pdf key", http.StatusBadRequest)
		return
	}

	// Resolve to the absolute filesystem path and double-check it
	// stays inside the configured root. filepath.Clean neutralises
	// any ".." tricks, then filepath.Rel expresses the diff from root.
	// If Rel starts with ".." or is empty, the path escapes the root.
	cleaned := filepath.Clean(filepath.Join(h.rootDir, key))
	absRoot, err := filepath.Abs(h.rootDir)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	absFull, err := filepath.Abs(cleaned)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	rel, err := filepath.Rel(absRoot, absFull)
	if err != nil || rel == "" || strings.HasPrefix(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	f, err := os.Open(absFull)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "pdf not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("open: %v", err), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		http.Error(w, fmt.Sprintf("stat: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	// Inline so the browser renders the PDF in the viewer tab
	// (rather than triggering a download). The UI's "Open PDF"
	// button uses window.open with _blank anyway.
	w.Header().Set("Content-Disposition", `inline; filename="`+filepath.Base(key)+`"`)
	w.Header().Set("Cache-Control", "private, max-age=300")

	http.ServeContent(w, r, filepath.Base(key), info.ModTime(), f)
}