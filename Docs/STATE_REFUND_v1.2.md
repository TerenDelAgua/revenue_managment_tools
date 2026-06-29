================================================================
TEREN HOTELS — REFUND v1.2 · STATE OF THE WORLD
================================================================
Purpose:    Snapshot of what's done, what's missing, and what
            just got fixed — for the refund flow specifically.
Author:     Continuation after fixing the room_id filter,
            invoice_number sync, and drawer→list refresh bugs.
Date:       June 2026
Spec:       docs/features/TEREN_Hotels_Invoicing_Spec_v1.2.md
Sister:     docs/HANDOFF_v1.2_backend_done_frontend_pending.md
            (still authoritative for the backend v1.2 narrative;
             THIS doc is the post-session truth + open work list.)

================================================================
1. WHERE WE ARE (TL;DR)
================================================================

Backend v1.2 is COMPLETE and end-to-end verified (migration 008
applied, smoke tests A–H passed). The frontend is ~70% done: the
core refund UX (picker, force_override modal, refund-all modal,
status pill) is in place and unit-tested. What's missing is the
PDF stamp, the ↩ / ⚠ icons on the badges, the daily summary KPI
adjustments, and a couple of edge cases the user has been
running into.

Hard numbers right now:
  backend tests:  go test ./...  → PASS
  frontend tests: pnpm test --run → 103/103 PASS
  typecheck:      pnpm check      → 0 errors, 0 warnings

Last 4 commits on feat_invoicing (newest first):
  a81ea49 fix(invoices)/list: auto-refresh after drawer mutation
  1b7d560 fix(invoicing)/number: self-healing counter on first insert
  69fccb6 fix(bookings)/list: accept room_id query filter
  733b839 feat(invoicing): add v1.2 refund features and bulk refunds

================================================================
2. WHAT'S DONE
================================================================

2.1  Backend (all done, see HANDOFF for full detail)

  ✅ Migration 008 — invoices.status='refunded', needs_review,
     payments.invalidated_*, refund_batches, trg_invoice_status
  ✅ Models — ForceOverride, InvalidatedAt/By/Reason, RefundAllInput
  ✅ Repository — R-07 gates (7 sentinel errors), RefundAll atomic,
     paymentAggCTE filters invalidated, effective_status='refunded'
  ✅ Service — RefundAll + canRefund auth + error mapping
  ✅ Handler — POST /api/v1/invoices/{id}/refund-all + force_override
  ✅ Routes — refund-all mounted in main.go

2.2  Frontend — refund UX (mostly done)

  ✅ BLOCK 7  — types.ts has the v1.2 fields:
                - Payment.remaining_reverseable, invalidated_*
                - InvoiceStatus / PaymentStatus include 'refunded'
                - InvoiceSummary.total_refunded, needs_review
                - RegisterPaymentPayload.force_override
  ✅ BLOCK 8  — PaymentForm.svelte picker UX:
                - lists cobrable payments (positive, not reversal,
                  not invalidated, remaining > 0)
                - click → pre-fills amount/method/reference/reversal_of
                - cap = remaining_reverseable ?? amount
                - method locked (gated by force_override flow)
                - "Refund another payment" banner via closeOnSuccess=false
                - 10 unit tests (PT-19…PT-28) PASS
  ✅ BLOCK 9  — common/ConfirmDestructive.svelte:
                - checkbox acknowledgement (no typing)
                - backdrop click + Escape = Cancel
                - focus-trap basics + ARIA dialog
                - wired into PaymentForm's "Change refund method" link
                - 8 unit tests (CD-01…CD-08) PASS
                - i18n namespaced as confirmDestructive.changeRefundMethod
                  and confirmDestructive.refundAll (pre-configured for B10)
  ✅ BLOCK 11 (partial) — invoice status pill + filter:
                - 'refunded' translated ("Refunded" / "Dikembalikan")
                - pill uses bg-error-subtle + text-error-base palette
                - InvoiceList filter dropdown includes 'refunded' option
                - MISSING: ↩ glyph on the pill (DS §5.4) — pure CSS,
                  tracked in §3 below

2.3  Bugs fixed this session (block refund testing, then continue)

  ✅ GET /bookings now honours room_id query param (handler +
     service + repo), smoke-tested with rooms 102 and 103.
  ✅ Drawer→list: InvoiceWidget's onChange now flows through
     InvoiceDrawer → /invoices/+page.svelte → listRef.refresh(),
     so register payment / void / refund / regen PDF in the
     drawer update the list behind it without manual reload.
  ✅ get_next_invoice_number self-healing on first INSERT — the
     function now derives the initial counter value from
     MAX(invoice_number) of existing rows, so a truncated counter
     can never re-issue a number that's already taken.

================================================================
3. WHAT'S MISSING (open work)
================================================================

3.1  BLOCK 10 — Refund all button (small)

  Status: backend ready (POST /refund-all), ConfirmDestructive
          pre-translated (confirmDestructive.refundAll), but the
          button + click handler are not yet in InvoiceWidget.svelte.

  Where: web/src/lib/components/invoice/InvoiceWidget.svelte
  Need:  - "Refund all payments" button visible when
           invoice.total_paid > 0 AND status != 'refunded' AND
           status != 'void' AND role === 'owner'
         - clicking opens <ConfirmDestructive> with the refundAll
           i18n namespace + body listing count + total amount
         - onConfirm → POST /api/v1/invoices/{id}/refund-all with
           { reason, force_override: false } → refetch invoice
         - handle backend errors (409 INVOICE_TERMINAL, 422
           INVALID_REFUND_TARGET, 404 NO_PAYMENTS_TO_REFUND)
  Tests: 1–2 unit tests on InvoiceWidget (button gating + click
         opens modal + confirm fires POST).

3.2  BLOCK 11 — ↩ and ⚠ glyphs on the status pill (small)

  Status: palette is correct (error-subtle / warning-subtle) but
          the spec §5.4 also asks for inline icons:
            refunded  → ↩ (U+21A9) on the pill
            needs_review → ⚠ (U+26A0) on the pill
          Currently shown as a coloured dot only.

  Where: InvoiceWidget.svelte (the statusStyle pill) +
         InvoiceList.svelte (table status column)
  Need:  - add inline `<span aria-hidden="true">↩</span>` /
           `<span aria-hidden="true">⚠</span>` next to the dot
         - keep the status text + i18n label; icon is decorative
  Tests: extend IL-08 + an InvoiceWidget test to assert the glyph
         is present.

3.3  BLOCK 12 — PDF REFUNDED diagonal stamp (backend + UI)

  Status: not started. Backend renders the VOID watermark the
          same way; REFUNDED needs the same treatment.

  Where: backend/pkg/pdfgen/generator.go (add a parallel branch
          on invoice.Status == 'refunded' that draws the diagonal
          stamp), plus a "Regenerate PDF" path so a re-issue
          picks up the new stamp (the existing button is wired
          but currently no-ops on already-refunded invoices).

  Need:  - draw stamp at 35° angle, ~96pt, red-50% opacity,
           font-weight 900, "REFUNDED"
         - same for needs_review: small inline "REVIEW NEEDED"
           ribbon at the top-right of the PDF
  Tests: visual; spec §6 + §13 (DIC-07).

3.4  BLOCK 13 — Daily summary + tax report UI refinements (medium)

  Status: backend already returns total_refunded + needs_review_count
          on the /reports/daily and /reports/monthly-tax endpoints,
          but the frontend doesn't render them.

  Where: web/src/routes/reports/+page.svelte (or whatever the
          current reports route is) + i18n keys.

  Need:  - new "Refunded" KPI card (−Rp total_refunded)
         - "Net revenue" = collected − refunded
         - warning banner if needs_review_count > 0 (link to
           /invoices?status=needs_review)
         - tax report monthly: net_tax (already in backend),
           refunded_count line
  Tests: 1 unit test on the report renderer.

3.5  E2E smoke + spec compliance test (small)

  Status: spec §10.1 enumerates the Go tests for the new
          behaviour (TestRefundOneToOne, TestRefundPartialAccumulation,
          TestRefundForceOverride, TestRefundInvalidated, TestRefundAll,
          TestOverRefundBlocked, TestInvoiceStatusTrigger,
          TestInvoiceStatusPrecedence, TestNeedsReviewExclusion,
          TestStatusMigration). None of those exist yet.

  Where: backend/internal/repository/*_test.go +
         backend/internal/service/*_test.go
  Need:  add at minimum:
         - TestRefundAll_InvTerminal (R-08)
         - TestRefundAll_Atomic (full tx: N refunds + 1 batch row)
         - TestForceOverride (R-07 method mismatch + override → OK)
         - TestStatusTriggerRefunded (cash basis math)

================================================================
4. THE 0004 GAP (info, no action)
================================================================

INV-2026-0004 does NOT exist in the database. The gap is real
and permanent (next invoice will be INV-2026-0006).

Cause: while debugging the duplicate-key bug, the developer ran
the SQL `DELETE FROM invoice_sequences; SELECT get_next_invoice_number(...)`
twice from psql. The SELECT is outside any booking-creation
transaction, so when it incremented the counter (no row → INSERT
VALUES(..., MAX+1) = 4), nothing rolled back. Later the user's
first real booking Test (28/6 09:50) consumed 0005 because the
counter was already at 4.

The fix (migration 009) prevents this from happening via the
normal booking flow — the counter + invoice insert share a tx,
so any failure rolls both back. The remaining gap is purely
from manual psql calls and is accepted as cosmetic.

================================================================
5. KNOWN EDGE CASES (not yet fixed)
================================================================

5.1  Refunding the LAST cobrable payment

  After my last refund in the smoke test, INV-2026-0002 went to
  status='refunded'. The backend correctly returns 409
  INVOICE_TERMINAL if the user tries to refund again. The frontend
  currently doesn't disable the "Refund" button when status is
  'refunded' — it would only show the error after click. Should
  hide the action altogether (cleaner UX).

5.2  needs_review invoices have no UI

  The backend sets needs_review=true on over-refunded rows (a row
  where the sum of valid refunds > the original charge). The
  InvoiceSummary carries the flag (since the v1.2 types work),
  but neither InvoiceWidget nor InvoiceList render anything for it.
  Tied to §3.2 (warning icon).

5.3  Bookings with total_amount = 0 (no charge yet)

  The new payment booking (Test Sync) had total_amount=0 and
  failed to create a guest (FK violation on guests_property_id_fkey
  — probably an unrelated bug in the smoke harness, NOT a refund
  bug). Invoice 0004 was never created for it. Such zero-charge
  invoices should probably be voided automatically at check-in
  to avoid accumulating "ghost" bookings in the UI. Block 14 will
  decide whether that's worth a spec change.

================================================================
6. KEY INVARIANTS (read before touching code)
================================================================

- invoices.status ∈ {active, void, refunded}; void and refunded
  are TERMINAL. Use status (not effective_status) for "can the
  user act?" checks.

- paymentAggCTE filters invalidated_at IS NULL. Refunds are
  attached to the non-invalidated original row, period.

- get_next_invoice_number is gapless within a (property, year)
  tuple under normal load. The §10 regression (1000 concurrent
  goroutines → 1..N) still holds after migration 009.

- force_override is a destructive UX choice: it requires the
  ConfirmDestructive modal acknowledgement. Backend enforces
  the audit trail note ("[OVERRIDE] method changed from X to Y
  by Z"), frontend enforces the modal.

- R-08 lifecycle is enforced server-side via trg_invoice_status
  and the gates in repo.RegisterPayment. The frontend only has
  to hide the actions; the server is the source of truth.

- File paths are absolute Windows-style. Use "file:///c:/TEREN/..."
  when citing links.

================================================================
7. CURRENT DEV DB STATE (29 Jun 2026)
================================================================

invoices table (only INV-2026-* and the new live ones):
  INV-2026-0001  status=void       total=555000
  INV-2026-0002  status=refunded   total=666000
  INV-2026-0003  status=void       total=0       (Eustaquio, was 0003 before)
  INV-2026-0005  status=refunded   total=555     (Test, room 202, last cycle)

There is no INV-2026-0004 — see §4.

invoice_sequences:
  next_number = 5 → next booking will be INV-2026-0006

room-status distribution: rooms 101/102/103/201/202 all have at
least one booking each (use GET /bookings?room_id=... to drill in
now that the filter is implemented).

================================================================
8. RECOMMENDED NEXT SESSION ORDER
================================================================

1. BLOCK 11 glyphs — pure cosmetic, 10 min.
2. BLOCK 10 refund-all button — small frontend addition + 1 test,
   20 min.
3. E2E smoke (BLOCK 14 subset): manual run-through
   create-booking → register payment → refund via picker →
   confirm force_override path → refund-all → check badge →
   regenerate PDF. Use INV-2026-0006 (the next booking) as a
   clean slate.
4. BLOCK 12 PDF REFUNDED stamp.
5. BLOCK 13 reports UI refinements.
6. BLOCK 14 full Go test suite per §10.1.

================================================================
9. OPEN QUESTIONS FOR THE USER
================================================================

- Should BLOCK 5.1 hide the Refund button outright when
  status in {refunded, void}? (I think yes.)
- Should BLOCK 5.3 auto-void zero-charge invoices at check-in?
  Or leave them as-is for the owner to review?
- Is BLOCK 3.5 worth implementing as Go tests now, or is the
  end-to-end manual smoke enough?

================================================================
END
================================================================