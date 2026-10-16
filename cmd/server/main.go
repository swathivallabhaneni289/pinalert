// Command server runs Pinalert's HTTP API. It never applies database
// migrations itself — see cmd/migrate — and it never generates a random
// SESSION_SECRET: on a free hosting tier that sleeps and cold-starts
// frequently, doing so would silently invalidate every visitor's session on
// every restart, and once Phase 2 makes sessions load-bearing for
// reputation, would destroy that continuity outright.
package main

import (
	"context"
	"log"
	"net/http"
	netmail "net/mail"
	"os"
	"strconv"
	"strings"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/geocode"
	"pinalert/internal/mailer"
	"pinalert/internal/moderation"
	"pinalert/internal/service"
	"pinalert/internal/session"
	"pinalert/internal/store"
	sqlcgen "pinalert/internal/store/sqlc"
)

const devInsecureSecret = "dev-only-insecure-secret-do-not-use-in-production"

// Fallback map centre for visitors who deny geolocation (Bengaluru), and the
// default nearby-reports search radius. Configurable here rather than
// hard-coded in JS — see handlers.PageConfig.
const (
	fallbackLat     = 12.9716
	fallbackLon     = 77.5946
	defaultRadiusKm = 10.0
)

// requestLinkRateLimitBurst/Every are DEC-I's per-IP budget for POST
// /api/auth/request-link (01.1-05-PLAN.md): a burst of 5 immediately, one
// token refilled every 60 seconds thereafter. Deliberately the looser,
// secondary control — PROJECT.md flags CGNAT (many legitimate Indian mobile
// users sharing one carrier IP) as a real collateral-damage risk, so the
// per-address cooldown in internal/service.AuthService.RequestLink carries
// the primary load.
const (
	requestLinkRateLimitBurst = 5
	requestLinkRateLimitEvery = 60 * time.Second
)

// geocodeRateLimitBurst/Every are this plan's per-IP secondary budget for
// GET /api/geocode: a burst of 3 immediately, one token refilled every 2
// seconds thereafter. Deliberately shaped differently from the
// request-link budget above: typing into a search box is bursty within
// one report submission and rare across a session (07-RESEARCH.md Open
// Question 2). The real D-02 policy ceiling is the process-wide
// one-request-per-second limiter inside internal/geocode, not this number.
const (
	geocodeRateLimitBurst = 3
	geocodeRateLimitEvery = 2 * time.Second
)

// placeRateLimitBurst/Every are the per account budget of GET
// /api/geocode/place, the reader's place search: a burst of 3, then one token
// every 4 seconds. Keyed by verified account, not by IP, because carrier NAT
// shares one address across many phones.
const (
	placeRateLimitBurst = 3
	placeRateLimitEvery = 4 * time.Second
)

// The D-11 budgets, the one named place for every Phase 4 limit number. Each
// bucket allows Burst at once and then one more every Every. The vote cooldown
// is burst 1: a second vote of one kind on one report waits a full Every. The
// submit attempt and network backstop shapes are research defaults that protect
// the screening provider and the server. api.RateLimitsDefault mirrors these
// values for Deps literals that leave a field zero. Each bucket's idle window
// follows from burst times refill (04-03).
const (
	reportSeriousBurst = 5
	reportSeriousEvery = 5 * time.Minute
	reportOtherBurst   = 8
	reportOtherEvery   = 15 * time.Minute
	voteBurst          = 20
	voteEvery          = 15 * time.Second
	voteChangeBurst    = 1
	voteChangeEvery    = 30 * time.Second
	flagBurst          = 10
	flagEvery          = 2 * time.Minute
	submitAttemptBurst = 20
	submitAttemptEvery = 6 * time.Second
	ipBackstopBurst    = 60
	ipBackstopEvery    = 20 * time.Second
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	env := os.Getenv("ENV")
	secret := loadSessionSecret(env)

	ctx := context.Background()
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	sessionMgr, err := session.NewManager([]byte(secret), env != "development")
	if err != nil {
		log.Fatalf("constructing session manager: %v", err)
	}

	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		log.Fatalf("parsing page template: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// BASE_URL roots the absolute verification links AuthService mints
	// (e.g. https://pinalert.example/auth/verify?token=...) — it must be the
	// externally reachable origin, not necessarily this process's own bind
	// address, so it defaults to a same-host guess only for local
	// development and should always be set explicitly in any real
	// deployment.
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	// LoadMailer selects the Resend-backed implementation when
	// RESEND_API_KEY is set, or the log-only development fallback
	// otherwise — failing fast outside ENV=development, the same
	// startup contract loadSessionSecret already establishes above. A
	// misconfigured deploy must fail loudly at boot rather than silently
	// accept logins nobody can complete.
	mail, err := mailer.LoadMailer(env)
	if err != nil {
		log.Fatalf("loading mailer: %v", err)
	}

	// LoadScreener selects the OpenAI moderation client when OPENAI_API_KEY is
	// set, a pass everything screener with a WARNING in development, and fails
	// fast otherwise, which is the startup contract the mailer follows. One
	// screener value is built per process, so its request and token budget is
	// process wide, and the background text check worker reuses this same
	// value. The key is never logged.
	screener, err := moderation.LoadScreener(env)
	if err != nil {
		log.Fatalf("loading text screener: %v", err)
	}

	assetVersion := strconv.FormatInt(time.Now().Unix(), 10)

	// NOMINATIM_CONTACT_EMAIL is operator-supplied configuration for the
	// address-search feature's outbound User-Agent to OSM Nominatim.
	// Deliberately NOT a log.Fatal path, unlike SESSION_SECRET and
	// RESEND_API_KEY above: those two fail fast outside ENV=development
	// because a misconfigured deploy would otherwise boot into a state
	// where nobody can log in at all, whereas address search is a
	// secondary convenience whose whole design contract (D-04) is that its
	// failure never blocks reporting. Failing to boot over it would invert
	// that contract. internal/geocode's userAgent falls back to the
	// project repository URL as the identifier when this is empty, so the
	// outbound agent string stays non-stock regardless.
	nominatimContactEmail := os.Getenv("NOMINATIM_CONTACT_EMAIL")
	if nominatimContactEmail == "" {
		log.Println("WARNING: NOMINATIM_CONTACT_EMAIL unset; outbound geocoding User-Agent will fall back to the project repository URL")
	}

	// CLIENT_IP_HEADER names the header the host overwrites with the visitor
	// address (CF-Connecting-IP on Render, Fly-Client-IP on Fly.io, X-Real-IP
	// on Railway). It turns on the network address backstop and makes every per
	// IP limiter key on that header. It is only safe on a host that overwrites
	// the header, so the canary check in the README checklist comes first.
	// Unset leaves the backstop off and the limits per account. A value that is
	// not a plain header name stops startup.
	clientIPHeader := strings.TrimSpace(os.Getenv("CLIENT_IP_HEADER"))
	if clientIPHeader == "" {
		log.Println("WARNING: CLIENT_IP_HEADER unset; the network address backstop is off and limits stay per account")
	} else {
		for _, c := range clientIPHeader {
			isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
			if !isLetter && !(c >= '0' && c <= '9') && c != '-' {
				log.Fatalf("CLIENT_IP_HEADER must be a plain header name of letters, digits and hyphens")
			}
		}
		log.Printf("client IP header for the network backstop: %s", clientIPHeader)
	}

	// The emergency numbers the unofficial bar prints come from one setting,
	// read once here (D-22). The loaded values are logged so the owner can
	// compare them with the government page before any public promotion.
	emergencyNumbers := handlers.LoadEmergencyNumbers(os.LookupEnv)
	log.Printf("emergency numbers: primary %s, older %q, state line %q, district line %q",
		emergencyNumbers.Primary, emergencyNumbers.Older, emergencyNumbers.StateHelpline, emergencyNumbers.DistrictHelpline)

	// CONTACT_EMAIL is the public contact address the Disclaimer and Policy
	// pages show (D-24). Unset means no contact line anywhere. A value that is
	// not a bare email address stops startup, so a broken mailto link never
	// reaches a public page.
	contactEmail := strings.TrimSpace(os.Getenv("CONTACT_EMAIL"))
	if contactEmail == "" {
		log.Println("WARNING: CONTACT_EMAIL unset; the Disclaimer and Policy pages show no contact line")
	} else if addr, err := netmail.ParseAddress(contactEmail); err != nil || addr.Name != "" || addr.Address != contactEmail {
		log.Fatalf("CONTACT_EMAIL is not a bare email address")
	}

	// MODERATOR_EMAILS is the server list of moderators (D-08). It is parsed
	// once here with the sign in normaliser. A bad entry stops startup with a
	// message that names the variable and the entry's position, never the
	// address (T-04-07). Unset or empty leaves nobody able to open the screen.
	moderatorEmails, err := service.ParseModeratorEmails(os.Getenv("MODERATOR_EMAILS"))
	if err != nil {
		log.Fatalf("MODERATOR_EMAILS: %v", err)
	}
	if len(moderatorEmails) == 0 {
		log.Printf("WARNING: MODERATOR_EMAILS is not set; nobody can open the moderator screen")
	}

	// This is the only construction site of the real geocode client in the
	// whole program, which is what makes the process-wide limiter inside
	// internal/geocode genuinely process wide. Both the report form's search
	// and the reader's place search are given this same value below.
	geocodeClient := geocode.NewClient(nominatimContactEmail)

	queries := sqlcgen.New(pool)
	deps := api.Deps{
		Session:     sessionMgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries, service.WithScreener(screener)),
		Votes:       service.NewVotingService(queries),
		AuthService: service.NewAuthService(queries, mail, baseURL),
		Auth: handlers.AuthConfig{
			AssetVersion: assetVersion,
		},
		Chrome:   handlers.Chrome{Numbers: emergencyNumbers, ContactEmail: contactEmail},
		Template: tmpl,
		// Parsed once from MODERATOR_EMAILS above; empty admits nobody.
		ModeratorEmails: moderatorEmails,
		// Keep and Remove lock the check row first inside one transaction,
		// the order the moderation worker takes (WR-S01).
		ModerationTx: service.PgxModerationTx(pool),
		Page: handlers.PageConfig{
			FallbackLat:     fallbackLat,
			FallbackLon:     fallbackLon,
			DefaultRadiusKm: defaultRadiusKm,
			VoteRadiusKm:    service.MaxVoteDistanceKm,
			AssetVersion:    assetVersion,
		},
		Dev: env == "development",
		// DEC-I: burst 5, one token refilled per 60 seconds, applied only to
		// POST /api/auth/request-link (see api.NewRouter). Named here, in
		// one place, rather than left as literals inside the router.
		RequestLinkRateLimit: api.RequestLinkRateLimit{
			Burst: requestLinkRateLimitBurst,
			Every: requestLinkRateLimitEvery,
		},
		// The one geocode client built above serves both lanes: the
		// report form's address search and the reader's place search.
		// Sharing the single value is what keeps the limiter inside
		// internal/geocode process wide, so the reader lane can never
		// push the program past the upstream policy ceiling. The
		// variable is a *geocode.Client that is never nil, so it is safe
		// to hand to the PlaceSearcher interface directly.
		Geocode:     geocodeClient,
		PlaceSearch: geocodeClient,
		GeocodeRateLimit: api.GeocodeRateLimit{
			Burst: geocodeRateLimitBurst,
			Every: geocodeRateLimitEvery,
		},
		// Burst 3, one token per 4 seconds, per verified account, applied
		// only to GET /api/geocode/place (see api.NewRouter).
		PlaceRateLimit: api.PlaceRateLimit{
			Burst: placeRateLimitBurst,
			Every: placeRateLimitEvery,
		},
		ClientIPHeader: clientIPHeader,
		// The seven D-11 shapes from the constants block above.
		RateLimits: api.RateLimits{
			ReportSerious: api.BucketLimit{Every: reportSeriousEvery, Burst: reportSeriousBurst},
			ReportOther:   api.BucketLimit{Every: reportOtherEvery, Burst: reportOtherBurst},
			Vote:          api.BucketLimit{Every: voteEvery, Burst: voteBurst},
			VoteChange:    api.BucketLimit{Every: voteChangeEvery, Burst: voteChangeBurst},
			Flag:          api.BucketLimit{Every: flagEvery, Burst: flagBurst},
			SubmitAttempt: api.BucketLimit{Every: submitAttemptEvery, Burst: submitAttemptBurst},
			IPBackstop:    api.BucketLimit{Every: ipBackstopEvery, Burst: ipBackstopBurst},
		},
	}
	router := api.NewRouter(deps)

	// The worker retries text checks that could not finish at submit: one sweep
	// at boot because the free host cold starts, then one every 30 seconds. The
	// claim's row lock and lease keep a second instance from double screening.
	moderationWorker := service.NewModerationWorker(queries, screener)
	go moderationWorker.Run(context.Background())

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("pinalert listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// loadSessionSecret reads SESSION_SECRET, failing fast unless env is
// "development" — the process must never auto-generate a random secret at
// startup (see package doc comment above).
func loadSessionSecret(env string) string {
	secret := os.Getenv("SESSION_SECRET")
	if secret != "" {
		return secret
	}
	if env != "development" {
		log.Fatal("SESSION_SECRET must be set outside ENV=development")
	}
	log.Println("WARNING: SESSION_SECRET unset; using insecure development fallback secret")
	return devInsecureSecret
}
