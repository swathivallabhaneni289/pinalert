// Command server runs Pinalert's HTTP API. It never applies database
// migrations itself — see cmd/migrate — and it never generates a random
// SESSION_SECRET: on a free hosting tier that sleeps and cold-starts
// frequently, doing so would silently invalidate every visitor's session on
// every restart, and once Phase 2 makes sessions load-bearing for
// reputation, would destroy that continuity outright.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	netmail "net/mail"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/areas"
	"pinalert/internal/geocode"
	"pinalert/internal/mailer"
	"pinalert/internal/moderation"
	"pinalert/internal/official"
	"pinalert/internal/push"
	"pinalert/internal/service"
	"pinalert/internal/session"
	"pinalert/internal/store"
	sqlcgen "pinalert/internal/store/sqlc"
)

const devInsecureSecret = "dev-only-insecure-secret-do-not-use-in-production"

// lateUpdates lets the phone alert dispatcher hold the updates service before it
// exists. The updates service needs the report service as its nearby reader,
// and the report service needs the dispatcher as its change notifier, so one of
// the three has to be filled in last. Until it is set, Refresh does nothing.
type lateUpdates struct {
	mu    sync.Mutex
	inner service.UpdatesRefresher
}

func (l *lateUpdates) set(u service.UpdatesRefresher) {
	l.mu.Lock()
	l.inner = u
	l.mu.Unlock()
}

func (l *lateUpdates) Refresh(ctx context.Context, accountID int64) error {
	l.mu.Lock()
	inner := l.inner
	l.mu.Unlock()
	if inner == nil {
		return nil
	}
	return inner.Refresh(ctx, accountID)
}

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

// geocodeRateLimitBurst/Every are the per account budget of GET /api/geocode,
// the report form's search: 8 searches at once, then one a second (quick
// 261016-sne; it was per IP, 3 then one every 2 seconds). Deliberately shaped
// differently from the request-link budget above: typing into a search box is
// bursty within one report submission and rare across a session. Sized for
// one person typing one address with a 300 ms pause between searches; see
// api.GeocodeRateLimitDefault for the reasoning. The real D-02 policy ceiling
// is the process wide limiter inside internal/geocode, not this number.
const (
	geocodeRateLimitBurst = 8
	geocodeRateLimitEvery = time.Second
)

// placeRateLimitBurst/Every are the per account budget of GET
// /api/geocode/place, the reader's place search: the same 8 at once, then one
// a second (it was 3, then one every 4 seconds). Keyed by verified account,
// not by IP, because carrier NAT shares one address across many phones.
const (
	placeRateLimitBurst = 8
	placeRateLimitEvery = time.Second
)

// The D-11 budgets, the one named place for every Phase 4 limit number. Each
// bucket allows Burst at once and then one more every Every. The vote cooldown
// is burst 1: a second vote of one kind on one report waits a full Every. The
// submit attempt shape is a research default that protects the screening
// provider; the network backstop has its own block below. api.RateLimitsDefault mirrors these
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
)

// The network backstop (WR-S02), keyed on the client address and on only when
// CLIENT_IP_HEADER is set. India's carrier NAT puts many phones behind one
// address, so each action family has its own bucket (votes cannot starve
// report posting) and each is sized so that about 100 accounts behind one
// address, each at its own per account limit above, never meet it. One
// machine flooding from one address through many accounts is still capped.
//   - Votes: burst 2000 = 100 x voteBurst 20; one more every 150 ms
//     (6.7 a second) = 100 x one per voteEvery 15 s.
//   - Reports: burst 2000 = 100 x submitAttemptBurst 20, which covers every
//     account's 5 serious plus 8 other posts with room for rejected tries;
//     one more every second, above the 100 accounts' combined posting refill
//     of 100 x (one per 5 minutes plus one per 15 minutes), about 0.44 a
//     second, and still a cap of 60 posts a minute from one address.
//   - Flags: burst 1000 = 100 x flagBurst 10; one more every 1.2 s =
//     100 x one per flagEvery 2 minutes.
//
// These are not owner numbers (D-11 sets only the account allowances); tune
// them from the block log, whose bucket names are ip_backstop_report,
// ip_backstop_vote and ip_backstop_flag. api.RateLimitsDefault mirrors them.
const (
	ipBackstopReportBurst = 2000
	ipBackstopReportEvery = time.Second
	ipBackstopVoteBurst   = 2000
	ipBackstopVoteEvery   = 150 * time.Millisecond
	ipBackstopFlagBurst   = 1000
	ipBackstopFlagEvery   = 1200 * time.Millisecond
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
	// address-search feature's outbound User-Agent to Photon and OSM
	// Nominatim (and Nominatim's email parameter).
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

	// The VAPID identity for phone notifications comes from the environment
	// only. Both keys unset leaves push off: the server runs as before and every
	// device reads unsupported. One key, a malformed key or a mismatched pair
	// stops startup, with a message that never holds a key (research Pitfall 15).
	pushCfg, pushOn, err := push.LoadConfig(os.Getenv)
	if err != nil {
		log.Fatalf("loading the VAPID configuration: %v", err)
	}
	if !pushOn {
		log.Println("WARNING: VAPID_PUBLIC_KEY and VAPID_PRIVATE_KEY unset; phone notifications are off")
	}

	// Posting and voting poke the dispatcher only when push is on. A nil
	// *PushDispatcher stored in the notifier interface would not be a nil
	// interface, so the options are added only inside this branch.
	reportOpts := []service.ReportOption{service.WithScreener(screener)}
	var voteOpts []service.VotingOption
	var pushDispatcher *service.PushDispatcher
	var pushUpdates *lateUpdates
	if pushOn {
		pushUpdates = &lateUpdates{}
		// The one geocode client is also the dispatcher's area namer (T-07-01).
		pushDispatcher = service.NewPushDispatcher(queries, pushUpdates, geocodeClient)
		reportOpts = append(reportOpts, service.WithReportChangeNotifier(pushDispatcher))
		voteOpts = append(voteOpts, service.WithVoteChangeNotifier(pushDispatcher))
	}
	reportService := service.NewReportService(queries, reportOpts...)
	voteService := service.NewVotingService(queries, voteOpts...)
	if pushOn {
		pushUpdates.set(service.NewUpdatesService(queries, reportService))
	}

	// The User-Agent NASA GIBS sees when a globe tile is fetched (plan 05-04): the
	// public address and the contact address, whichever are set, else the
	// repository URL the geocoder falls back to.
	globeUserAgent := "Pinalert/1.0 (+https://github.com/swathivallabhaneni289/pinalert)"
	if publicURL := os.Getenv("BASE_URL"); publicURL != "" || contactEmail != "" {
		switch {
		case publicURL != "" && contactEmail != "":
			globeUserAgent = "Pinalert/1.0 (+" + publicURL + "; contact " + contactEmail + ")"
		case publicURL != "":
			globeUserAgent = "Pinalert/1.0 (+" + publicURL + ")"
		default:
			globeUserAgent = "Pinalert/1.0 (contact " + contactEmail + ")"
		}
	}

	// The official alerts service (plan 05-08): reads the stored NDMA SACHET
	// alerts for a reader and refreshes them lazily. It is built once here, and
	// the later Phase 5 plans add their hooks and feeds to it through options.
	// The address passed is BASE_URL as set, not the localhost fallback above,
	// because it only goes into the User-Agent NDMA sees.
	officialSvc := official.NewService(pool, queries, official.Config{
		ContactEmail: contactEmail,
		BaseURL:      os.Getenv("BASE_URL"),
		// With push off no worker would ever send or skip official push rows,
		// so none are queued (IN-S11 of the Phase 5 review).
		PushOff: !pushOn,
	})

	// OFFICIAL_REFRESH_TOKEN is the bearer secret of the external cron's refresh
	// endpoint (plan 05-12, D-15). Unset leaves the endpoint a 404, so official
	// alerts refresh only while the server is awake. A value under 32 characters
	// stops startup; the message never holds the value.
	officialRefreshToken := strings.TrimSpace(os.Getenv("OFFICIAL_REFRESH_TOKEN"))
	if err := handlers.ValidateRefreshToken(officialRefreshToken); err != nil {
		log.Fatalf("%v", err)
	}
	if officialRefreshToken == "" {
		log.Println("WARNING: OFFICIAL_REFRESH_TOKEN unset; the external refresh endpoint is off and official alerts refresh only while the server is awake")
	}

	deps := api.Deps{
		Official:    officialSvc,
		Session:     sessionMgr,
		Sessions:    queries,
		Reports:     reportService,
		Votes:       voteService,
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
		// Delete my account scrubs, deletes and revokes in one transaction.
		AccountTx: service.PgxAccountTx(pool),
		Page: handlers.PageConfig{
			FallbackLat:     fallbackLat,
			FallbackLon:     fallbackLon,
			DefaultRadiusKm: defaultRadiusKm,
			VoteRadiusKm:    service.MaxVoteDistanceKm,
			AssetVersion:    assetVersion,
			// The VAPID public key, empty when push is off. The pages and the
			// subscription route both read it from here.
			PushPublicKey: pushCfg.PublicKey,
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
		// Burst 8, one token a second, per verified account, applied only
		// to GET /api/geocode/place (see api.NewRouter).
		PlaceRateLimit: api.PlaceRateLimit{
			Burst: placeRateLimitBurst,
			Every: placeRateLimitEvery,
		},
		ClientIPHeader: clientIPHeader,
		// Sent to NASA GIBS when a sign in globe tile is fetched (plan 05-04).
		GlobeUserAgent: globeUserAgent,
		// The bearer secret of the external cron's refresh endpoint, read above;
		// empty keeps the endpoint a 404 (plan 05-12).
		OfficialRefreshToken: officialRefreshToken,
		// The first-time flow gate is on (D-27, orchestrator decision 5). Every
		// account that has not agreed to the current terms version or not
		// finished the flow is sent to /welcome for page requests; existing
		// accounts, the owner's included, see it once and no consent is
		// backfilled. /api/, /welcome, the legal pages, the login and verify
		// pages and every non GET request pass untouched, so the offline queue
		// never meets a new refusal.
		OnboardingGate: true,
		// The six D-11 account shapes and the three network backstop shapes
		// from the constants blocks above.
		RateLimits: api.RateLimits{
			ReportSerious:    api.BucketLimit{Every: reportSeriousEvery, Burst: reportSeriousBurst},
			ReportOther:      api.BucketLimit{Every: reportOtherEvery, Burst: reportOtherBurst},
			Vote:             api.BucketLimit{Every: voteEvery, Burst: voteBurst},
			VoteChange:       api.BucketLimit{Every: voteChangeEvery, Burst: voteChangeBurst},
			Flag:             api.BucketLimit{Every: flagEvery, Burst: flagBurst},
			SubmitAttempt:    api.BucketLimit{Every: submitAttemptEvery, Burst: submitAttemptBurst},
			IPBackstopReport: api.BucketLimit{Every: ipBackstopReportEvery, Burst: ipBackstopReportBurst},
			IPBackstopVote:   api.BucketLimit{Every: ipBackstopVoteEvery, Burst: ipBackstopVoteBurst},
			IPBackstopFlag:   api.BucketLimit{Every: ipBackstopFlagEvery, Burst: ipBackstopFlagBurst},
		},
	}
	router := api.NewRouter(deps)

	// The worker retries text checks that could not finish at submit: one sweep
	// at boot because the free host cold starts, then one every 30 seconds. The
	// claim's row lock and lease keep a second instance from double screening.
	moderationWorker := service.NewModerationWorker(queries, screener)
	go moderationWorker.Run(context.Background())

	// The official feeds refresh while the host is awake: one sweep at boot
	// because the free host cold starts, then one every five minutes (plan 05-12,
	// D-15). Each feed goes through RefreshIfDue, so its minimum gap and single
	// flight still hold. A sleeping host refreshes only when a request, usually the
	// external cron's call to the refresh endpoint, wakes it.
	go officialSvc.Run(context.Background())

	// The phone alert dispatcher queues notifications and the worker sends them.
	// Each sweeps once at boot because the free host cold starts. The claim
	// lease, renewed just before each row, and the lease conditional marks keep a
	// second instance from sending or marking a row the first one is working.
	// Delivery is still at least once: a crash after a push service accepted a
	// message and before the mark sends it again once the lease passes. A
	// sleeping host sends nothing until a request wakes it (decision 9, review
	// W2 and N5). One endpoint policy value is shared by the sender and the
	// worker, so both read a device's host the same way.
	//
	// Both loops run on runCtx, which SIGINT and SIGTERM end, and the process
	// waits for them before it exits, so a row whose send a push service
	// accepted reaches its mark on the worker's WithoutCancel write context
	// instead of being sent again after a redeploy (review S-05, 2026-10-25).
	runCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	// Once the first signal ends runCtx, give SIGINT and SIGTERM back their
	// default behaviour, so a second Ctrl-C during the shutdown grace ends the
	// process at once instead of being swallowed (skeptic IN-01).
	context.AfterFunc(runCtx, stopSignals)
	var pushRunners []func(context.Context)
	if pushOn {
		pushPolicy := push.DefaultEndpointPolicy()
		pushSender := push.NewWebPushSender(pushCfg, push.NewGuardedClient(), pushPolicy)
		pushWorker := service.NewPushWorker(queries, pushSender, service.WithPushWorkerPolicy(pushPolicy))
		// The official lane of the phone channel (plan 05-16, D-11): it sends
		// what plan 05-11 queued for Severe and Extreme official alerts, from the
		// same sender and policy values, with one sweep at boot. It is a runner
		// next to the 3.3 worker, so the shutdown waits for its current row too.
		// Nothing runs when VAPID is off.
		officialPushWorker := service.NewOfficialPushWorker(queries, pushSender, areas.Default(), service.WithOfficialPushWorkerPolicy(pushPolicy))
		pushRunners = append(pushRunners,
			pushDispatcher.Run,
			pushWorker.Run,
			officialPushWorker.Run,
		)
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("pinalert listening on :%s", port)
	if err := serveUntilDone(runCtx, srv.ListenAndServe, srv.Shutdown, shutdownGrace, pushRunners...); err != nil {
		log.Fatalf("server error: %v", err)
	}
	log.Printf("pinalert stopped")
}

// shutdownGrace bounds how long the process waits after SIGINT or SIGTERM for
// requests in flight and for the push worker's current row: one fan out
// (service.PushSendTimeout), then its marks (the worker's own write timeout,
// also 10 seconds), with a little room to spare.
const shutdownGrace = service.PushSendTimeout + 15*time.Second

// serveUntilDone starts each runner on a context that ends with ctx, runs
// serve, and when ctx ends calls shutdown and waits for the runners to return,
// all within grace. A serve error other than http.ErrServerClosed stops the
// runners too and comes back. A runner still busy after grace is reported as
// an error rather than waited for. Returns nil after a clean stop.
func serveUntilDone(ctx context.Context, serve func() error, shutdown func(context.Context) error, grace time.Duration, runners ...func(context.Context)) error {
	runCtx, stopRunners := context.WithCancel(ctx)
	defer stopRunners()
	var wg sync.WaitGroup
	for _, run := range runners {
		wg.Add(1)
		go func() {
			defer wg.Done()
			run(runCtx)
		}()
	}

	served := make(chan error, 1)
	go func() { served <- serve() }()

	var errs []error
	select {
	case err := <-served:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	case <-ctx.Done():
		log.Printf("pinalert stopping")
	}
	stopRunners()

	graceCtx, cancel := context.WithTimeout(context.Background(), grace)
	defer cancel()
	if err := shutdown(graceCtx); err != nil {
		errs = append(errs, fmt.Errorf("shutting down: %w", err))
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-graceCtx.Done():
		errs = append(errs, errors.New("background loops still busy after the shutdown grace period"))
	}
	return errors.Join(errs...)
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
