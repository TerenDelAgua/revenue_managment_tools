package pdfgen

// Labels is the set of user-visible strings rendered in the invoice PDF.
// Everything is ASCII-clean so the bundled Helvetica font (WinAnsi)
// renders without mojibake — see generator.go's sanitize() for context.
//
// Spec note: the PDF follows the SPA's locale per the UX team's
// localisation rules. The handler reads Accept-Language and picks the
// matching Labels set. New languages need an entry in labelsByLocale.
type Labels struct {
	Title          string // "INVOICE" / "FACTURA" / "FAKTUR"
	IssuedBy       string
	GuestDetails   string
	CheckIn        string
	CheckOut       string
	Room           string
	Description    string
	Qty            string
	Price          string
	Total          string
	Subtotal       string
	Tax            string // VAT/PPN
	Payments       string
	IssuedAt       string // "Issued: %s"
	By             string // "By: %s"
	ThankYou       string
	RefundSuffix   string // "(refund)"
	Void           string
	// Refunded (v1.2 Block 12): stamp text painted diagonally across
	// the centre of the PDF when lifecycle='refunded'. Mirrors Void
	// so an auditor scans both terminal states identically.
	Refunded       string
	BookingLabel   string // "Booking: %s"
	CurrencySuffix string // "IDR - UTC+8"
}

// labelsByLocale is the source of truth for PDF copy. Only the locales
// declared here are honoured; anything else falls back to English.
//
// To add a language: copy the en block, replace the strings, and add
// the new key here. The unit test asserts every locale has every
// required field.
var labelsByLocale = map[string]Labels{
	"en": {
		Title:          "INVOICE",
		IssuedBy:       "Issued by",
		GuestDetails:   "Guest details",
		CheckIn:        "Check-in",
		CheckOut:       "Check-out",
		Room:           "Room",
		Description:    "Description",
		Qty:            "Qty",
		Price:          "Price",
		Total:          "Total",
		Subtotal:       "Subtotal",
		Tax:            "PPN",
		Payments:       "Payments",
		IssuedAt:       "Issued",
		By:             "By",
		ThankYou:       "Thank you for your stay.",
		RefundSuffix:   "(refund)",
		Void:           "VOID",
		Refunded:       "REFUNDED",
		BookingLabel:   "Booking",
		CurrencySuffix: "IDR - UTC+8",
	},
	"es": {
		Title:          "FACTURA",
		IssuedBy:       "Emitida por",
		GuestDetails:   "Datos del huésped",
		CheckIn:        "Check-in",
		CheckOut:       "Check-out",
		Room:           "Habitación",
		Description:    "Descripción",
		Qty:            "Cant",
		Price:          "Precio",
		Total:          "Total",
		Subtotal:       "Subtotal",
		Tax:            "PPN",
		Payments:       "Pagos registrados",
		IssuedAt:       "Emitida",
		By:             "Por",
		ThankYou:       "Gracias por su estancia.",
		RefundSuffix:   "(reembolso)",
		Void:           "ANULADA",
		Refunded:       "REEMBOLSADA",
		BookingLabel:   "Reserva",
		CurrencySuffix: "IDR - UTC+8",
	},
	"id": {
		Title:          "FAKTUR",
		IssuedBy:       "Diterbitkan oleh",
		GuestDetails:   "Detail tamu",
		CheckIn:        "Check-in",
		CheckOut:       "Check-out",
		Room:           "Kamar",
		Description:    "Keterangan",
		Qty:            "Jumlah",
		Price:          "Harga",
		Total:          "Total",
		Subtotal:       "Subtotal",
		Tax:            "PPN",
		Payments:       "Pembayaran",
		IssuedAt:       "Diterbitkan",
		By:             "Oleh",
		ThankYou:       "Terima kasih atas kunjungannya.",
		RefundSuffix:   "(pengembalian)",
		Void:           "DIBATALKAN",
		Refunded:       "DIKEMBALIKAN",
		BookingLabel:   "Pemesanan",
		CurrencySuffix: "IDR - UTC+8",
	},
}

// LabelsFor returns the labels for a locale string. Falls back to
// English when:
//   - locale is empty
//   - locale is not a known tag (e.g. "en-GB" → "en")
//   - locale is a region variant we don't ship yet
//
// We use a simple prefix match ("es-MX" → "es") because the SPA's
// available locales are just "en", "es", "id" — we don't want to pull
// in golang.org/x/text/language for a 3-entry map.
func LabelsFor(locale string) Labels {
	if locale == "" {
		return labelsByLocale["en"]
	}
	// Strip region / script suffixes after the primary subtag.
	if len(locale) >= 2 {
		primary := locale[:2]
		if l, ok := labelsByLocale[primary]; ok {
			return l
		}
	}
	return labelsByLocale["en"]
}