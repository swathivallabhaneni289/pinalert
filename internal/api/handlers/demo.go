package handlers

import (
	"time"

	"github.com/go-chi/chi/v5"

	"pinalert/internal/demo"
)

// DemoLimit is one per IP token bucket: Burst requests at once, then one more
// every Every.
type DemoLimit struct {
	Every time.Duration
	Burst int
}

// DemoConfig configures DemoRoutes.
type DemoConfig struct {
	Store      *demo.Store
	VisitLimit DemoLimit
	WriteLimit DemoLimit
}

// Defaults, generous because carrier NAT shares addresses.
var (
	DemoVisitLimitDefault = DemoLimit{Every: 6 * time.Second, Burst: 30}
	DemoWriteLimitDefault = DemoLimit{Every: time.Second, Burst: 60}
)

// DemoRoutes is a stub until the Demo API lands.
func DemoRoutes(cfg DemoConfig) chi.Router {
	return chi.NewRouter()
}
