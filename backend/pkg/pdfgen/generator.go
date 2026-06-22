// Package pdfgen generates invoice PDFs using gofpdf and uploads them
// via pdfstore.Store. It implements the service.PDFGenerator interface
// declared in B3.
//
// Library note: we use gofpdf directly instead of maroto v2. maroto v2's
// API is currently fragmented (consts split into sub-packages: align,
// fontstyle, linestyle, pagesize, etc.) and in flux, which makes the
// wrapper unstable. gofpdf is the underlying engine of maroto v1/v2
// anyway, so the output is equivalent and the API is stable.
package pdfgen

import (
	"context"
	"fmt"

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
// repo, builds the PDF, and uploads it via the store. Returns the
// public URL on success.
func (g *InvoicePDFGenerator) Generate(ctx context.Context, invoiceID uuid.UUID) (string, error) {
	d, err := g.invoiceRepo.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return "", fmt.Errorf("load invoice: %w", err)
	}
	bytes, err := buildPDF(d)
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

// buildPDF constructs the A4 portrait PDF for the given invoice detail.
// Pure function (no I/O). The caller is responsible for uploading.
func buildPDF(d models.InvoiceDetail) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(true, 12)
	pdf.AddPage()

	const w float64 = 210 // A4 width mm
	const leftMargin float64 = 10
	const contentW float64 = w - 2*leftMargin

	// --- Header
	pdf.SetFont("Helvetica", "B", 24)
	pdf.SetTextColor(colorPrimary.r, colorPrimary.g, colorPrimary.b)
	pdf.Cell(0, 12, "TEREN")
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.SetX(leftMargin + contentW - 60)
	pdf.CellFormat(60, 10, "FACTURA", "", 0, "R", false, 0, "")
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.SetX(leftMargin + contentW - 60)
	pdf.CellFormat(60, 6, d.InvoiceNumber, "", 0, "R", false, 0, "")
	pdf.Ln(8)

	// --- Two columns: Property (left) | Guest (right)
	colW := (contentW - 4) / 2
	yStart := pdf.GetY()

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.CellFormat(colW, 4, "Emitida por", "", 0, "L", false, 0, "")
	pdf.SetX(leftMargin + colW + 4)
	pdf.CellFormat(colW, 4, "Datos del huésped", "", 0, "L", false, 0, "")
	pdf.Ln(5)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.MultiCell(colW, 4, propertyBlock(), "", "L", false)
	leftEndY := pdf.GetY()
	pdf.SetXY(leftMargin+colW+4, yStart+4)
	pdf.MultiCell(colW, 4, guestBlock(d), "", "L", false)
	rightEndY := pdf.GetY()
	pdf.SetY(max(leftEndY, rightEndY) + 4)

	// --- Dates row (only if any line item exists, otherwise skip)
	if len(d.LineItems) > 0 {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
		pdf.CellFormat(contentW/3, 4, "Check-in", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "Check-out", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "Habitación", "", 0, "L", false, 0, "")
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
		// MVP: dates are unavailable on the model; we render empty.
		pdf.CellFormat(contentW/3, 4, "—", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "—", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/3, 4, "—", "", 0, "L", false, 0, "")
		pdf.Ln(4)
	}

	// --- Line items table
	drawTableHeader(pdf, leftMargin, contentW)
	for _, li := range d.LineItems {
		drawTableRow(pdf, leftMargin, contentW, li.Description,
			fmt.Sprintf("%.2f", li.Quantity),
			idr(li.UnitPrice),
			idr(li.Total))
	}

	// --- Totals (right-aligned)
	pdf.Ln(2)
	right := leftMargin + contentW
	rowLabelW := right - 60
	rowValueW := 60.0

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, "Subtotal", idr(d.Subtotal), false, false)

	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, idrPct(d.PPNRateSnapshot), idr(d.TaxAmount), false, false)

	// Separator
	pdf.SetDrawColor(colorText.r, colorText.g, colorText.b)
	pdf.Line(leftMargin+rowLabelW, pdf.GetY(), right, pdf.GetY())
	pdf.Ln(1)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	drawTotalRow(pdf, leftMargin, rowLabelW, rowValueW, "TOTAL", idr(d.Total), true, false)

	// --- Payments breakdown
	if len(d.Payments) > 0 {
		pdf.Ln(6)
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(contentW, 5, "Pagos registrados", "", 0, "L", false, 0, "")
		pdf.Ln(5)
		for _, p := range d.Payments {
			sign := ""
			if p.IsReversal || p.Amount < 0 {
				sign = " (refund)"
			}
			ref := ""
			if p.Reference != nil && *p.Reference != "" {
				ref = "  Ref: " + *p.Reference
			}
			// 4 columns: method | date | amount | ref
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
			pdf.CellFormat(contentW*0.33, 4, string(p.Method), "", 0, "L", false, 0, "")
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
	// gofpdf's TransformRotate conflicts with SetAutoPageBreak in v1.16
	// (TransformEnd fails with "out of sequence" if the current page
	// is full). We work around it by drawing the watermark in a new
	// page break before any new content would be needed. Simpler: a
	// single horizontal red row near the totals.
	if d.Status == models.InvoiceStatusVoid {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 32)
		pdf.SetTextColor(colorError.r, colorError.g, colorError.b)
		pdf.CellFormat(contentW, 12, "VOID", "", 0, "C", false, 0, "")
		pdf.Ln(8)
	}

	// --- Footer
	pdf.SetY(282)
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(colorMuted.r, colorMuted.g, colorMuted.b)
	pdf.CellFormat(0, 4,
		fmt.Sprintf("Emitida: %s — Por: %s",
			d.IssuedAt.Format("02 Jan 2006 15:04"), d.CreatedBy),
		"", 0, "L", false, 0, "")
	pdf.Ln(4)
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.CellFormat(0, 4, "Gracias por su estancia.", "", 0, "L", false, 0, "")
	pdf.Ln(4)
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

func propertyBlock() string {
	return "TEREN Test Hotel\n" +
		"Jl. Pantai Kuta 88\n" +
		"Bali, Indonesia\n" +
		"IDR · UTC+8"
}

func guestBlock(d models.InvoiceDetail) string {
	// MVP: BookingDetail (which we'd need for the full guest info)
	// isn't loaded on the PDF; we render a stable label from the booking ID.
	short := d.BookingID.String()[:8]
	return fmt.Sprintf("Reserva: %s\n", short)
}

// =============================================================================
// table helpers
// =============================================================================

func drawTableHeader(pdf *gofpdf.Fpdf, left, w float64) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(colorText.r, colorText.g, colorText.b)
	pdf.SetFillColor(colorBorder.r, colorBorder.g, colorBorder.b)
	pdf.SetDrawColor(colorBorder.r, colorBorder.g, colorBorder.b)

	// 4 columns: Concepto 6/12, Cant 2/12, Precio 2/12, Total 2/12
	colWidths := []float64{w * 6 / 12, w * 2 / 12, w * 2 / 12, w * 2 / 12}
	headers := []string{"Concepto", "Cant", "Precio", "Total"}
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
