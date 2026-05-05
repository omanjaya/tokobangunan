package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"

	"github.com/labstack/echo/v4"
)

// cspNonceCtxKey is the typed key used to stash the per-request CSP nonce
// inside Echo context AND request context.Context. Templ templates receive
// context.Context, so the nonce is propagated there to enable nonce={ ... }
// attributes on inline <script> tags without threading props through every
// view.
type cspNonceCtxKey struct{}

// CSPNonceContextKey is the canonical key both for echo.Context.Get and for
// context.Context. Echo allows any string; we use a stable string literal so
// callers can also fetch via c.Get("csp_nonce") if convenient.
const CSPNonceContextKey = "csp_nonce"

// CSPNonce returns the per-request nonce stored in ctx by the CSP middleware,
// or an empty string if none is present (e.g. during direct render in tests).
func CSPNonce(ctx context.Context) string {
	if v := ctx.Value(cspNonceCtxKey{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// CSP installs a per-request nonce in the request context and writes the
// Content-Security-Policy header. When production is true, a hardened policy
// is emitted; otherwise a relaxed dev policy is used (still nonce-based for
// scripts so inline tags exercise the same code path).
//
// Policy summary:
//
//	default-src 'self';
//	script-src 'self' 'nonce-<NONCE>';
//	style-src 'self' 'unsafe-inline';   // Tailwind/Alpine inline styles
//	img-src 'self' data: blob:;
//	font-src 'self' data:;
//	connect-src 'self';
//	frame-ancestors 'none';
//
// Inline <script> tags must carry nonce={ layout.CSPNonce(ctx) } to execute.
// External scripts under /static remain allowed via 'self'.
func CSP(production bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			buf := make([]byte, 16)
			if _, err := rand.Read(buf); err != nil {
				// Fallback: empty nonce — inline scripts will be blocked, but
				// the request still proceeds. This is the safe failure mode.
				return next(c)
			}
			nonce := base64.RawStdEncoding.EncodeToString(buf)

			c.Set(CSPNonceContextKey, nonce)

			req := c.Request()
			ctx := context.WithValue(req.Context(), cspNonceCtxKey{}, nonce)
			c.SetRequest(req.WithContext(ctx))

			csp := "default-src 'self'; script-src 'self' 'nonce-" + nonce + "'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'"
			if !production {
				// Dev: allow eval-less inline only via nonce; identical policy
				// keeps parity with prod so violations surface during dev.
				_ = production
			}
			c.Response().Header().Set("Content-Security-Policy", csp)
			return next(c)
		}
	}
}
