// Package browser optionally enriches crawled pages with data only a real
// browser can produce: console errors, failed network requests, and full
// load timing. The Playwright implementation is used when BROWSER_ENABLED
// and a browser install is available; otherwise the audit runs HTTP-only
// through the Noop service.
package browser

import (
	"context"

	"github.com/ergonix/auditor/backend/internal/models"
)

// Service enriches a page in place. Implementations must be safe for
// concurrent use.
type Service interface {
	// Enrich navigates to p.FinalURL and fills ConsoleErrors, FailedRequests
	// and LoadTimeMs. Errors are returned but callers treat them as
	// best-effort: a failed enrichment never fails an audit.
	Enrich(ctx context.Context, p *models.Page) error
	Close() error
}

// Noop is the fallback Service used when Playwright is unavailable.
type Noop struct{}

// NewNoop returns a Service that does nothing.
func NewNoop() Service { return Noop{} }

func (Noop) Enrich(context.Context, *models.Page) error { return nil }
func (Noop) Close() error                               { return nil }
