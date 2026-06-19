================================================================
TEREN HOTELS — INVOICING HANDOFF
================================================================
Purpose:    Resume document for the next session.
Author:     Continuation from the v1.2 working session.
Date:       June 2026
State:      Backend R-07 + R-08 DONE & verified end-to-end.
            Frontend (blocks 7-14) PENDING.
Spec:       docs/features/TEREN_Hotels_Invoicing_Spec_v1.2.md (RATIFIED)
This file:  docs/HANDOFF_v1.2_backend_done_frontend_pending.md

================================================================
1. WHERE WE ARE (TL;DR)
================================================================

The invoicing feature was developed in this order:
- B0 — DS v1.1 + themeStore + i18n bootstrap          ✅
- B1 — DB schema + seeds + schema_test                ✅
- B2 — Repository (12 methods)                        ✅
- B3 — Service (atomic, refunds, idempotency)         ✅
- B4 — Handlers (9 endpoints + middleware)            ✅
- B5 — PDF gen (gofpdf + R2 + LocalStore + HTTP)      ✅
- B6 — Invoice widget (drawer del booking)            ✅
- B7 — Payment form (Idempotency-Key, X-User-ID)      ✅
- B8 — Daily summary + Tax report + CSV export        ✅
- B9 — Invoice list + filters + drawer                ✅
- B10 — Invoice list export (paginado interno)        ✅
- B11 — Refunds UI (mode toggle + force_override)     ✅
- Refunds fix (reversal_of)                           ✅
- Void fix (post-void crash, empty-reason UX)         ✅
- Refund auth (DEV_OVERRIDE_ROLE)                     ✅

Then v1.2 was ratified with 2 new ratifications:
- R-07 — Refund 1:1 (validation gates)
- R-08 — Refunded status lifecycle

The backend implementation of R-07 + R-08 is COMPLETE and verified
end-to-end (migration applied, smoke tests passed, all gates return
correct HTTP codes). The frontend work for v1.2 has NOT been started.

================================================================
2. WHAT'S DONE (Backend v1.2)
================================================================

2.1  Migration: backend/migrations/008_invoice_refunded_status.up.sql
       APPLIED to dev DB. It does:
       - ALTER invoices.status CHECK → ('active','void','refunded')
       - ADD invoices.needs_review BOOLEAN DEFAULT FALSE
       - ADD payments.invalidated_at / invalidated_by / invalidated_reason
       - CREATE refund_batches table
       - CREATE FUNCTION trg_invoice_status_update + trigger
       - DATA CLEANUP: invalidate 2 legacy rows (a94e74eb... and
         0ac15330...), flip already-refunded invoices to status='refunded',
         and flag needs_review=true on over-refunded ones.

2.2  Models: backend/internal/models/invoicing.go
       - InvoiceStatusRefunded, PaymentStatusRefunded (new enum values)
       - Invoice.NeedsReview bool
       - Payment.InvalidatedAt / InvalidatedBy / InvalidatedReason
       - RegisterPaymentInput.ForceOverride bool (R-07)
       - RefundAllInput, RefundBatch (new types)

2.3  Repository: backend/internal/repository/invoice_repository.go
       - 6 new sentinel errors for R-07 gates:
         ErrInvoiceTerminal, ErrRefundNotReverse, ErrRefundCrossInvoice,
         ErrRefundOfRefund, ErrRefundExceedsCap, ErrRefundMethodMismatch,
         ErrRefundOverRefund
       - paymentAggCTE filters invalidated_at IS NULL
       - effectiveStatusExpr adds 'refunded' in the precedence:
         void > refunded > paid > partial > unpaid
       - RegisterPayment: 7 R-07 gates (cross-invoice, refund-of-refund,
         cap, method, over-refund, target not found, target invalidated)
       - New method: RefundAll(ctx, RefundAllInput) → RefundAllResult
         (atomic: N refunds + 1 refund_batches row in a single tx,
          with FOR UPDATE on the invoice and each target payment)
       - DailySummary / MonthlyTaxReport: cash basis (refund nets PPN
         of refund month, not invoice month), net_tax formula,
         needs_review exclusions

2.4  Service: backend/internal/service/invoice_service.go
       - New method: RefundAll(ctx, RefundAllInput, userRole)
         (calls canRefund for authorization, maps errors to BusinessError)

2.5  Handler: backend/internal/api/invoice_handler.go
       - New handler: RefundAll
         POST /api/v1/invoices/{id}/refund-all
         Body: { "reason": "...", "force_override": false }
         Returns: 200 with refunded_payments[], refund_batch_id,
                  total_refunded, invoice_lifecycle_after
       - RegisterPayment now accepts force_override in the body
       - RegisterPayment maps repo errors:
           - ErrInvoiceTerminal   → 409 INVOICE_TERMINAL
           - ErrRefund*           → 422 INVALID_REFUND_TARGET
           - ErrInvalidPayment + amount<0 → 422 INVALID_REFUND_TARGET

2.6  Routes: backend/cmd/api/main.go
       - Added: r.Post("/{id}/refund-all", invoiceHandler.RefundAll)

================================================================
3. STATE OF THE DEV DATABASE
================================================================

The dev DB (teren_hotels in Docker) has these post-migration values:

  invoices:
    - 2 legacy refund rows on INV-2026-0002 have been invalidated
      (a94e74eb... -100000 SLIP-R-001, 0ac15330... -666000 TRF-DEV-002).
      They stay visible with strikethrough + reason tooltip, but are
      excluded from total_paid / total_refunded / effective_status.
    - INV-2026-0002 itself was flipped to status='refunded' AFTER the
      smoke test ran (RefundAll was triggered). needs_review=false.
      Note: the spec says a refund-all that reaches total triggers
      status='refunded' via the trg_invoice_status_update trigger.

  payments on INV-2026-0002 (live, non-invalidated):
    - 07d6ca91... +300000 bank_transfer TRF-DEV-002
    - 5ef75b9b... +366000 cash (no ref)
    - 6c2aee68... -100000 cash REFUND-CASH-001 (manual test)
    - d7b3de6e... -266000 cash REFUND-{id[:8]} (auto via refund-all)
    - a19825a2... -300000 bank_transfer REFUND-TRF-DEV-002 (auto)
    (Note: total 666k paid vs 666k refunded → exactly fully refunded,
     so status='refunded' was set by the trigger.)

  refund_batches:
    - 1 row: e64fe1aa... reason="guest cancelled - smoke test"
             payment_ids={d7b3de6e, a19825a2} total_refunded=566000

================================================================
4. WHAT'S PENDING (Frontend v1.2 — blocks 7-14)
================================================================

These are the frontend changes required by the spec v1.2 sections:

BLOCK 7  Response shape: backend now returns
          - Invoice.NeedsReview
          - Payment.InvalidatedAt / InvalidatedBy / InvalidatedReason
          - Payment.RemainingReverseable (per-cobro, computed in API)
          - Refund lifecycle + effective_status='refunded'
        The frontend TYPES in web/src/lib/types.ts must include the new
        fields. The InvoiceDetail interface should add:
            invalidated_at?: string | null;
            invalidated_reason?: string | null;
            remaining_reverseable?: number | null;
            needs_review: boolean;
            lifecycle: 'active' | 'void' | 'refunded';

BLOCK 8  Frontend picker de pagos para refund (R-07, no formulario libre)
        Refactor PaymentForm.svelte:
          - When mode='refund', instead of free-form fields, show a
            LIST of cobrable payments (timeline) with method, date,
            original amount, available to refund.
          - Click on a payment opens the prefilled refund form
            (method locked to original, amount cap=remaining_reverseable,
             reference prefilled as REFUND-{original.reference},
             reversal_of set from the chosen payment).
          - Button "Refund another payment" if more payments remain.
        Files: web/src/lib/components/invoice/PaymentForm.svelte,
               web/src/lib/components/invoice/InvoiceWidget.svelte.

BLOCK 9  Modal de force_override (R-09 Q1: checkbox, no typing required)
        Show a destructive modal with:
          - Label: "Change refund method?"
          - Body: explains audit trail impact
          - Checkbox: "I understand this changes the audit trail"
          - Cancel / Confirm
        New file: web/src/lib/components/common/ConfirmDestructive.svelte
        Use: when user toggles "Change method" in the refund form.

BLOCK 10 Botón "Refund all" + modal de confirmación
        On InvoiceWidget: new action button when invoice.total_paid > 0
          and lifecycle='active'.
        Modal: "This refunds N payments totalling Rp X. Continue?"
        Calls POST /api/v1/invoices/{id}/refund-all.
        After success: refetch invoice; status will be 'refunded'.
        Files: InvoiceWidget.svelte, ConfirmDestructive.svelte.

BLOCK 11 Status badges 'refunded' + 'needs_review' (R-09 Q4)
        Per spec §5.4: bg-error-subtle + text-error-base + ↩ icon
        for refunded; bg-warning-subtle + ⚠ icon for needs_review.
        Files: web/src/lib/components/invoice/InvoiceList.svelte,
               web/src/lib/components/invoice/InvoiceWidget.svelte,
               web/src/lib/components/invoice/InvoiceDrawer.svelte.

BLOCK 12 PDF sello diagonal "REFUNDED" (mirrors VOID watermark)
        backend/pkg/pdfgen/generator.go needs to render a diagonal
        REFUNDED stamp when invoice.status='refunded'.
        Same treatment as VOID watermark per spec §6.

BLOCK 13 Daily summary + tax report UI refinements
        The frontend reports page should display:
          - new "Refunded" KPI card with -Rp total_refunded
          - "Net revenue" = collected - refunded
          - "needs_review_count" warning if > 0
        Tax report monthly:
          - net_tax (cash basis)
          - refunded_count line
        Files: web/src/routes/reports/* + i18n keys.

BLOCK 14 E2E smoke + spec compliance test
        - Test the picker UI: selecting a payment prefills correctly.
        - Test the destructive modal: checkbox disabled until checked.
        - Test the refund-all flow end-to-end with playwright.

================================================================
5. KEY INVARIANTS (read before touching code)
================================================================

- DB Schema (006 + 008): invoices.status ∈ {active,void,refunded},
  payments.invalidated_at is the marker for retired rows.

- R-07: refund MUST reference ONE original payment via reversal_of.
  Multiple payments → multiple refunds OR POST /refund-all.

- R-08: status='refunded' is TERMINAL. No payments or refunds can be
  added. Re-fetch after writes always (same pattern as the void fix).

- R-09 Q1: force_override UX is a CHECKBOX, no typing required.
- R-09 Q2: legacy -666000 + -100000 SLIP-R-001 rows are INVALIDATED,
  not deleted. Owner can re-issue correct refunds via the new picker.
- R-09 Q3: cash basis — refunds net PPN of the refund month, not the
  invoice month. No carry-forward between months.
- R-09 Q4: refunded badge = error-subtle + ↩ (NOT warning-*).

- DEV auth (still in place): the API client sends X-User-Role: 'owner'
  via DEV_OVERRIDE_ROLE in web/src/lib/api/client.ts. This is a dev
  override; production sources role from JWT.

- File paths are absolute Windows-style. Use "file:///c:/TEREN/..."
  when citing links.

================================================================
6. FILES TOUCHED IN v1.2 BACKEND
================================================================

NEW:
  backend/migrations/008_invoice_refunded_status.up.sql
  docs/features/TEREN_Hotels_Invoicing_Spec_v1.2.md

MODIFIED:
  backend/internal/models/invoicing.go
    (enum + NeedsReview + InvalidatedAt/B y/Reason + ForceOverride
     + RefundAllInput + RefundBatch)
  backend/internal/repository/invoice_repository.go
    (sentinel errors, R-07 gates, RefundAll, DailySummary/MonthlyTaxReport
     with cash basis + needs_review exclusions)
  backend/internal/service/invoice_service.go
    (RefundAll service method + error mapping)
  backend/internal/api/invoice_handler.go
    (RefundAll handler + force_override in body + R-07 error mapping)
  backend/cmd/api/main.go
    (POST /refund-all route)

DELETED: (none)

================================================================
7. TESTS
================================================================

Backend: `go test ./...` passes (api / repository / service / invoicing
/ pdfgen). The new R-07 / R-08 behaviour is exercised by smoke tests
against the live dev DB (see section 8 below). Service-level tests for
the new RefundAll method + RegisterPayment R-07 gates should be added
in a follow-up; see block 9 in section 4.

Frontend: pnpm test still at 77/77 (the v1.1 frontend state). All v1.2
changes will require new tests + updated existing ones.

================================================================
8. SMOKE TEST VERIFICATION (what we ran)
================================================================

These are the curl-based checks against the running API at port 8080.
The exact UUIDs are the real ones from dev DB so you can replay them:

  A) Refund individual 1:1 (cash 366k → -100k contra 5ef75b9b...)
     Expected: 201 Created. ✅
  B) Cross-invoice (UUID fake 00000000-...0099)
     Expected: 422 INVALID_REFUND_TARGET. ✅
  C) Method mismatch (qris vs bank_transfer), sin force_override
     Expected: 422 INVALID_REFUND_TARGET. ✅
  D) Refund-de-refund (target = previous refund row 6c2aee68...)
     Expected: 422 INVALID_REFUND_TARGET. ✅
  E) Cap exceeded (-400000 contra target 300000)
     Expected: 422 INVALID_REFUND_TARGET. ✅
  F) POST /refund-all reason="guest cancelled - smoke test"
     Expected: 200 OK, status pasa a refunded. ✅
  G) refund_batches audit row created with 2 payment_ids. ✅
  H) Reintentar /refund-all → 409 INVOICE_TERMINAL. ✅

The full smoke session log is in the conversation transcript. Re-run
locally with:
  1. Start API: cd backend && go build -o ./bin/teren-api.exe cmd/api/main.go
     && ./bin/teren-api.exe
  2. Pick a non-refunded invoice from the seed (INV-2026-0001 or 0002).
  3. Use the curl recipes in section 8 of the spec §10.1.

================================================================
9. RESUMING THE WORK (next session)
================================================================

Recommended order (matches section 4):

1. Read this file + the spec v1.2 sections relevant to your block.
2. Block 7: update types.ts with the new fields. Run `pnpm check`.
3. Block 8: refactor PaymentForm.svelte to show the picker UI in
   refund mode. Use existing dev data (INV-2026-0002 now has
   5 payments you can play with).
4. Block 9: create ConfirmDestructive.svelte modal. Wire to refund
   method override.
5. Block 10: add "Refund all" button + confirmation modal.
6. Block 11: add badges 'refunded' + 'needs_review' (error palette +
   ↩ / ⚠ icons).
7. Block 12: PDF REFUNDED stamp in pkg/pdfgen/generator.go.
8. Block 13: reports UI refinements (i18n keys en/es/id).
9. Block 14: tests + smoke verification (PNPM check, vitest run, go
   test, manual E2E with the API running).

After each block: pnpm check + pnpm test + go test ./... should pass.

================================================================
10. CONTACT / QUESTIONS
================================================================

If unsure about a design decision: read the spec v1.2 first. All R-07,
R-08, R-09 decisions are documented there with rationale. The spec
section structure is:
- §3 BR-INV-010 / BR-INV-011 (refund) / BR-INV-013 (refunded status)
- §4.3 (RegisterPayment validation order) / §4.12 (RefundAll endpoint)
- §4.8 (response shape) / §4.9 (daily summary) / §4.11 (tax report)
- §5.2 (refund picker UX) / §5.4 (badges)
- §6 (PDF REFUNDED stamp)
- §9 (new error codes: INVALID_REFUND_TARGET, INVOICE_TERMINAL,
      NO_PAYMENTS_TO_REFUND)
- §10.1 (new unit tests: TestRefundOneToOne, TestRefundPartialAccumulation,
      TestRefundForceOverride, TestRefundInvalidated, TestRefundAll,
      TestOverRefundBlocked, TestInvoiceStatusTrigger,
      TestInvoiceStatusPrecedence, TestNeedsReviewExclusion,
      TestStatusMigration)
- §11 (migration 008 details)
- §13 DIC-06/07/08 (DS candidates for v1.2)

If the next session needs to reset the dev DB state for clean testing,
use:
  docker exec teren-hotels-db psql -U teren -d teren_hotels \
    -c "DELETE FROM payments WHERE invoice_id='4e9d683d-3bb7-403c-bf42-05c31056ce72';"
Then reseed from cmd/seed if needed.

================================================================
END OF HANDOFF
================================================================