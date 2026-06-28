// Package middleware provides HTTP middleware for the API layer.
//
// Currently exported:
//   - IdempotencyKey: extracts the Idempotency-Key header (UUID) and
//     stores it in the request context. Handlers can then read it via
//     FromContext(r.Context()).
//
// This is intentionally minimal — production auth (JWT/session) is
// handled by a separate middleware in a future iteration. The handler
// layer reads user_id / role from X-User-ID / X-User-Role headers for
// now (dev convenience).
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxIdempotencyKey ctxKey = iota
	ctxUserID
	ctxUserRole
)

// IdempotencyKey is a chi-compatible middleware that reads the
// `Idempotency-Key` request header. If present, it MUST be a valid
// UUID; otherwise the request is rejected with 400 (defensive — saves
// the user from a confusing service error later). If absent, the
// handler proceeds and may choose to require it explicitly.
func IdempotencyKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}
		parsed, err := uuid.Parse(key)
		if err != nil {
			http.Error(w, `{"code":"INVALID_IDEMPOTENCY_KEY","message":"Idempotency-Key must be a UUID"}`, http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			return
		}
		ctx := context.WithValue(r.Context(), ctxIdempotencyKey, parsed)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FromContext returns the Idempotency-Key stored in ctx (if any) and a
// boolean indicating whether one was present.
func IdempotencyKeyFromContext(ctx context.Context) (*uuid.UUID, bool) {
	v := ctx.Value(ctxIdempotencyKey)
	if v == nil {
		return nil, false
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return nil, false
	}
	return &id, true
}

// AuthContext is a dev-only middleware that reads X-User-ID and X-User-Role
// headers and stashes them in the context. Real auth (JWT/session) is out
// of scope for B4 — production will replace this with proper middleware
// that validates a signed token and pulls user/role from claims.
func AuthContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if v := r.Header.Get("X-User-ID"); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				ctx = context.WithValue(ctx, ctxUserID, id)
			}
		}
		if role := r.Header.Get("X-User-Role"); role != "" {
			ctx = context.WithValue(ctx, ctxUserRole, role)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user ID, if any.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	v := ctx.Value(ctxUserID)
	if v == nil {
		return uuid.Nil, false
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

// UserRoleFromContext returns the authenticated user role, if any.
func UserRoleFromContext(ctx context.Context) (string, bool) {
	v := ctx.Value(ctxUserRole)
	if v == nil {
		return "", false
	}
	r, ok := v.(string)
	return r, ok
}
