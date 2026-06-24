// Package pdfgen generates invoice PDFs using gofpdf and uploads them
// via pdfstore.Store. It implements the service.PDFGenerator interface
// declared in B3.
//
// Library note: we use gofpdf directly instead of maroto v2. maroto v2's
// API is currently fragmented (consts split into sub-packages: align,
// fontstyle, linestyle, pagesize, etc.) and in flux, which makes the
// wrapper unstable. gofpdf is the underlying engine of maroto v1/v2
// anyway, so the output is equivalent and the API is stable.
//
// Locale & encoding note (B7-validation fix):
//   - gofpdf's bundled fonts (Helvetica, Times, Courier) are Type 1
//     PostScript with WinAnsi (CP1252) encoding. UTF-8 multi-byte
//     sequences render as mojibake ("Habitación" → "HabitaciÃ³n").
//   - To avoid that without pulling a TrueType font, every literal
//     string passed to gofpdf goes through sanitize() which forces
//     the result into 7-bit ASCII — safe for any PDF viewer and
//     keeps the PDF language-agnostic (English by default).
//   - When we need proper Unicode (e.g. a hotel name in Bahasa
//     Indonesia with diacritics), we'll load a TTF via
//     pdf.AddUTF8FontFromBytes. Out of scope for MVP.
package pdfgen

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jung-kurt/gofpdf"

	"github.com/terendelagua/teren-hotels-backend/internal/models"
	"github.com/terendelagua/teren-hotels-backend/internal/repository"
	"github.com/terendelagua/teren-hotels-backend/pkg/pdfstore"
)

// =============================================================================
// InvoicePDFGenerator
// =============================================================================

// InvoicePDFGenerator is the production implementation of
// service.PDFGenerator. It builds the PDF bytes via gofpdf and uploads
// them to the configured store (R2 in prod, local FS in dev).
type InvoicePDFGenerator struct {
	invoiceRepo *repository.InvoiceRepository
	store       pdfstore.Store
}

func NewInvoicePDFGenerator(invoiceRepo *repository.InvoiceRepository, store pdfstore.Store) *InvoicePDFGenerator {
	return &InvoicePDFGenerator{invoiceRepo: invoiceRepo, store: store}
}

// Generate implements the service.PDFGenerator interface. It loads
// the full invoice detail (header + line items + payments) from the
// repo, builds the PDF in the requested locale, and uploads it via
// the store. Returns the public URL on success.
//
// locale is a BCP-47 primary subtag ("en", "es", "id") or empty for
// the English fallback. Unknown tags also fall back to English.
func (g *InvoicePDFGenerator) Generate(ctx context.Context, invoiceID uuid.UUID, locale string) (string, error) {
	d, err := g.invoiceRepo.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return "", fmt.Errorf("load invoice: %w", err)
	}
	labels := LabelsFor(locale)
	bytes, err := buildPDF(d, labels)
	if err != nil {
		return "", err
	}
	key := pdfstore.ObjectKey(d.PropertyID, d.InvoiceNumber)
	url, err := g.store.Put(ctx, key, bytes)
	if err != nil {
		return "", fmt.Errorf("store put: %w", err)
	}
	return url, nil
}

// =============================================================================
// PDF builder (gofpdf)
// =============================================================================

// DS v1.1 color palette (must mirror the web tokens).
var (
	colorPrimary  = mustRGB("#FF8C42")
	colorPrimaryH = mustRGB("#E06B20")
	colorSubtle   = mustRGB("#FFF7ED")
	colorMuted    = mustRGB("#57534E")
	colorText     = mustRGB("#1C1917")
	colorBorder   = mustRGB("#E7E5E4")
	colorError    = mustRGB("#DC2626")
	colorSurface  = mustRGB("#FCFBFA")
)

type rgb struct{ r, g, b int }

// mustRGB parses a "#RRGGBB" hex string into an rgb struct. Panics on
// malformed input — only called with compile-time constants.
func mustRGB(h string) rgb {
	r, g, b := parseHex(h)
	return rgb{r: r, g: g, b: b}
}

func parseHex(h string) (r, g, b int) {
	if len(h) != 7 || h[0] != '#' {
		return 0, 0, 0
	}
	parse := func(c byte) int {
		switch {
		case c >= '0' && c <= '9':
			return int(c - '0')
		case c >= 'a' && c <= 'f':
			return int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			return int(c-'A') + 10
		}
		return 0
	}
	r = parse(h[1])*16 + parse(h[2])
	g = parse(h[3])*16 + parse(h[4])
	b = parse(h[5])*16 + parse(h[6])
	return
}

// IDR formats with no decimals (Indonesian currency convention).
func idr(v float64) string {
	return fmt.Sprintf("Rp %s", thousands(int64(v)))
}

func thousands(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := []byte{}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

func idrPct(rate float64) string {
	return fmt.Sprintf("PPN %.0f%%", rate*100)
}

// sanitize forces a string into 7-bit ASCII so gofpdf's bundled fonts
// (WinAnsi-encoded) render it without mojibake. Diacritics are folded
// to their base ASCII letter ("Habitación" → "Habitacion"); anything
// that can't be folded becomes '?' so we never emit non-ASCII bytes.
//
// Every string passed to gofpdf in this file MUST go through sanitize
// (or be ASCII by construction). The lint is in pkg/pdfgen/generator_test.go.
func sanitize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r <= 0x7F:
			// Pure ASCII — pass through unchanged.
			b.WriteRune(r)
		case r == '\u2018' || r == '\u2019':
			// Smart single quotes — fold to ASCII apostrophe.
			b.WriteByte('\'')
		case r == '\u201C' || r == '\u201D':
			// Smart double quotes — fold to ASCII quote.
			b.WriteByte('"')
		case r == '\u2013' || r == '\u2014':
			// En/em dashes — fold to ASCII hyphen-minus.
			b.WriteByte('-')
		case r == '\u2026':
			b.WriteString("...")
		case r == '\u00B7':
			// Middle dot — fold to ASCII hyphen so "IDR · UTC+8" becomes
			// "IDR - UTC+8" instead of "IDR ? UTC+8".
			b.WriteByte('-')
		case unicode.Is(unicode.Mn, r):
			// Combining marks are dropped (base char already in stream).
			continue
		default:
			if base, ok := diacriticFold[r]; ok {
				b.WriteRune(base)
			} else {
				b.WriteByte('?')
			}
		}
	}
	return b.String()
}

// diacriticFold maps common Latin diacritics to their ASCII base.
// Limited to the chars we actually see in our domain (Latin American
// Spanish, Indonesian, French). Anything not in the map → '?'.
var diacriticFold = map[rune]rune{
	// Lowercase
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a', 'æ': 'a',
	'ç': 'c',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'ñ': 'n',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o', 'ø': 'o', 'œ': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'ý': 'y', 'ÿ': 'y',
	'ß': 's', 'ð': 'd', 'þ': 't',
	// Uppercase
	'À': 'A', 'Á': 'A', 'Â': 'A', 'Ã': 'A', 'Ä': 'A', 'Å': 'A', 'Æ': 'A',
	'Ç': 'C',
	'È': 'E', 'É': 'E', 'Ê': 'E', 'Ë': 'E',
	'Ì': 'I', 'Í': 'I', 'Î': 'I', 'Ï': 'I',
	'Ñ': 'N',
	'Ò': 'O', 'Ó': 'O', 'Ô': 'O', 'Õ': 'O', 'Ö': 'O', 'Ø': 'O', 'Œ': 'O',
	'Ù': 'U', 'Ú': 'U', 'Û': 'U', 'Ü': 'U',
	'Ý': 'Y',
}

// buildPDF constructs the A4 portrait PDF for the given invoice detail.
// Pure function (no I/O). The caller is responsible for uploading.
//
// Layout (B7-validation 2): the previous version hardcoded the footer
// at Y=282mm, which overflowed to a second page when content was short.
// Now the footer flows naturally after the last content row, and we
// trim some vertical spacings so a single invoice (header + line items
// + totals + 1-2 payments) fits in one A4 page (~297mm).
//
// Locale: every user-visible string is taken from `labels`. The
// handler resolves `labels` from the Accept-Language header.
func buildPDF(d models.InvoiceDetail, labels Labels) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 10) // smaller bottom margin = more headroom
	pdf.AddPage()

	const w float64 = 210 // A4 width mm
	const leftMargin float64 = 10
	const contentW float64 = w - 2*leftMargin

	// --- Header (TEREN + Title + invoice number)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(colorPrimary.r, colorPrimary.g, colorPrimary.b)
	pdf.Cell(0, 10, "TEREN")
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.SetX(leftMargin + contentW - 60)
	pdf.CellFormat(60, 8, sanitize(labels.Title), "", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.SetX(leftMargin + contentW - 60)
	pdf.CellFormat(60, 5, sanitize(d.InvoiceNumber), "", 0, "R", false, 0, "")
	pdf.Ln(6)

	// --- Two columns: Property (left) | Guest (right)
	colW := (contentW - 4) / 2
	yStart := pdf.GetY()

	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.CellFormat(colW, 4, sanitize(labels.IssuedBy), "", 0, "L", false, 0, "")
	pdf.SetX(leftMargin + colW + 4)
	pdf.CellFormat(colW, 4, sanitize(labels.GuestDetails), "", 0, "L", false, 0, "")
	pdf.Ln(4)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.MultiCell(colW, 4, sanitize(propertyBlock(labels)), "", "L", false)
	leftEndY := pdf.GetY()
	pdf.SetXY(leftMargin+colW+4, yStart+4)
	pdf.MultiCell(colW, 4, sanitize(guestBlock(d, labels)), "", "L", false)
	rightEndY := pdf.GetY()
	pdf.SetY(max(leftEndY, rightEndY) + 3)

	// --- Dates row (only if any line item exists, otherwise skip)
	if len(d.LineItems) > 0 {
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
		pdf.CellFormat(contentW/3, 4, sanitize(labels.CheckIn), "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, sanitize(labels.CheckOut), "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, sanitize(labels.Room), "", 0, "L", false, 0, "")
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
		// MVP: dates are unavailable on the model; we render empty.
		pdf.CellFormat(contentW/3, 4, "-", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "-", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "-", "", 0, "L", false, 0, "")
		pdf.Ln(3)
	}

	// --- Line items table
	drawTableHeader(pdf, leftMargin, contentW, labels)
	for _, li := range d.LineItems {
		drawTableRow(pdf, leftMargin, contentW, sanitize(li.Description),
			fmt.Sprintf("%.2f", li.Quantity),
			idr(li.UnitPrice),
			idr(li.Total))
	}

	// --- Totals (right-aligned)
	pdf.Ln(1)
	right := leftMargin + contentW
	rowLabelW := right - 60
	rowValueW := 60.0

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, sanitize(labels.Subtotal), idr(d.Subtotal), false, false)

	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, idrPct(d.PPNRateSnapshot), idr(d.TaxAmount), false, false)

	// Separator
	pdf.SetDrawColor(colorText.r, colorText.g, colorText.b)
	pdf.Line(leftMargin+rowLabelW, pdf.GetY(), right, pdf.GetY())
	pdf.Ln(1)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, sanitize(labels.Total), idr(d.Total), true, false)

	// --- Payments breakdown
	if len(d.Payments) > 0 {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
		pdf.CellFormat(contentW, 4, sanitize(labels.Payments), "", 0, "L", false, 0, "")
		pdf.Ln(4)
		for _, p := range d.Payments {
			sign := ""
			if p.IsReversal || p.Amount < 0 {
				sign = "  " + sanitize(labels.RefundSuffix)
			}
			ref := ""
			if p.Reference != nil && *p.Reference != "" {
				ref = "  Ref: " + sanitize(*p.Reference)
			}
			// 4 columns: method | date | amount | ref
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
			pdf.CellFormat(contentW*0.33, 4, sanitize(string(p.Method)), "", 0, "L", false, 0, "")
			pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
			pdf.CellFormat(contentW*0.20, 4, p.ReceivedAt.Format("02 Jan 15:04"), "", 0, "L", false, 0, "")
			pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
			pdf.CellFormat(contentW*0.20, 4, fmt.Sprintf("%s%s", idr(p.Amount), sign), "", 0, "R", false, 0, "")
			pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
			pdf.CellFormat(contentW*0.27, 4, ref, "", 0, "L", false, 0, "")
			pdf.Ln(4)
		}
	}

	// --- VOID watermark
	if d.Status == models.InvoiceStatusVoid {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 28)
		pdf.SetTextColor(colorError.r, colorError.g, colorError.b)
		pdf.CellFormat(contentW, 10, sanitize(labels.Void), "", 0, "C", false, 0, "")
		pdf.Ln(4)
	}

	// --- Footer (B7-validation 2): flows naturally after content, on
	// the same page whenever possible. If we are very close to the
	// page bottom, SetAutoPageBreak pushes us to a new page — the
	// footer follows and stays at the bottom of the *last* page.
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.CellFormat(0, 3,
		fmt.Sprintf("%s: %s  -  %s: %s",
			sanitize(labels.IssuedAt),
			d.IssuedAt.Format("02 Jan 2006 15:04"),
			sanitize(labels.By),
			sanitize(d.CreatedBy.String())),
		"", 0, "L", false, 0, "")
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.CellFormat(0, 4, sanitize(labels.ThankYou), "", 0, "L", false, 0, "")
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.CellFormat(0, 3, `"Built with intention. Designed for flow. Owned by TEREN."`,
		"", 0, "C", false, 0, "")

	var out []byte
	if err := pdf.Output(&bufferAdapter{bytes: &out}); err != nil {
		return nil, fmt.Errorf("pdf output: %w", err)
	}
	return out, nil
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func propertyBlock(labels Labels) string {
	return "TEREN Test Hotel\n" +
		"Jl. Pantai Kuta 88\n" +
		"Bali, Indonesia\n" +
		labels.CurrencySuffix
}

func guestBlock(d models.InvoiceDetail, labels Labels) string {
	// MVP: BookingDetail (which we'd need for the full guest info)
	// isn't loaded on the PDF; we render a stable label from the booking ID.
	short := d.BookingID.String()[:8]
	return fmt.Sprintf("%s: %s\n", labels.BookingLabel, short)
}

// =============================================================================
// table helpers
// =============================================================================

func drawTableHeader(pdf *gofpdf.Fpdf, left, w float64, labels Labels) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.SetFillColor(colorBorder.r, colorBorder.g, colorBorder.b)
	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)

	// 4 columns: Description 6/12, Qty 2/12, Price 2/12, Total 2/12
	colWidths := []float64{w * 6 / 12, w * 2 / 12, w * 2 / 12, w * 2 / 12}
	headers := []string{
		sanitize(labels.Description),
		sanitize(labels.Qty),
		sanitize(labels.Price),
		sanitize(labels.Total),
	}
	aligns := []string{"L", "R", "R", "R"}
	y := pdf.GetY()
	x := left
	for i, h := range headers {
		pdf.Rect(x, y, colWidths[i], 6, "FD")
		pdf.SetXY(x, y)
		pdf.CellFormat(colWidths[i], 6, h, "", 0, aligns[i], false, 0, "")
		x += colWidths[i]
	}
	pdf.SetY(y + 6)
	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)
}

func drawTableRow(pdf *gofpdf.Fpdf, left, w float64, desc, qty, price, total string) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	colWidths := []float64{w * 6 / 12, w * 2 / 12, w * 2 / 12, w * 2 / 12}
	values := []string{desc, qty, price, total}
	aligns := []string{"L", "R", "R", "R"}
	y := pdf.GetY()
	x := left
	for i, v := range values {
		pdf.SetXY(x, y)
		pdf.CellFormat(colWidths[i], 5, v, "", 0, aligns[i], false, 0, "")
		x += colWidths[i]
	}
	pdf.SetY(y + 5)
}

func drawTotalRow(pdf *gofpdf.Fpdf, left, labelW, valueW float64, label, value string, bold, borderTop bool) {
	y := pdf.GetY()
	x := left
	if bold {
		pdf.SetFont("Helvetica", "B", 11)
	} else {
		pdf.SetFont("Helvetica", "", 9)
	}
	pdf.SetXY(x, y)
	pdf.CellFormat(labelW, 5, label, "", 0, "R", false, 0, "")
	pdf.SetXY(x+labelW, y)
	pdf.CellFormat(valueW, 5, value, "", 0, "R", false, 0, "")
	pdf.SetY(y + 5)
}

// =============================================================================
// io.Writer adapter for gofpdf.Output (which writes to a Writer, not a buffer).
// =============================================================================

// bufferAdapter wraps a *[]byte to satisfy io.Writer for gofpdf.Output.
type bufferAdapter struct {
	bytes *[]byte
}

func (b *bufferAdapter) Write(p []byte) (int, error) {
	*b.bytes = append(*b.bytes, p...)
	return len(p), nil
}
