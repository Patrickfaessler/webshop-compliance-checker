// Command api serves the REST API described in specs/08-api.yaml and, in this interim setup,
// runs the scan worker in the same process (it moves to cmd/worker once PostgreSQL + a job queue exist).
package main

import (
	"context"
	"crypto/rand"
	"log"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"compliancechecker/internal/api"
	"compliancechecker/internal/config"
	"compliancechecker/internal/crawler"
	"compliancechecker/internal/fetch"
	"compliancechecker/internal/pgstore"
	"compliancechecker/internal/ratelimit"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a1"
	"compliancechecker/internal/rules/a2"
	"compliancechecker/internal/rules/a3"
	"compliancechecker/internal/rules/a4"
	"compliancechecker/internal/rules/a6"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/ssrf"
	"compliancechecker/internal/worker"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	salt := make([]byte, 32) // rotates on every start (N4)
	if _, err := rand.Read(salt); err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Settings from the environment, optionally filled from deploy/.env (ADR-5). Real env vars win.
	if used, err := config.LoadDotEnv(config.DefaultDotEnvPaths...); err != nil {
		log.Fatalf("reading %s: %v", used, err)
	} else if used != "" {
		log.Printf("settings loaded from %s", used)
	}

	// ADR-6: scans are not stored; they stay in memory for scan.Retention (1 hour), the user downloads the report.
	// The PostgreSQL store (ADR-5) is only used when explicitly switched on with STORE=postgres.
	var store scan.Store
	switch mode := os.Getenv("STORE"); mode {
	case "postgres":
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			log.Fatal("STORE=postgres needs DATABASE_URL")
		}
		openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		pg, err := pgstore.Open(openCtx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer pg.Close()
		store = pg
		log.Printf("store: PostgreSQL (migrations applied), scans kept for %v", scan.Retention)
		go purgeLoop(ctx, func() (int, error) { n, err := pg.Purge(ctx); return int(n), err })
	case "", "memory":
		mem := scan.NewMemoryStore()
		store = mem
		log.Printf("store: memory only, scans are not saved and deleted after %v (ADR-6)", scan.Retention)
		go purgeLoop(ctx, func() (int, error) { return mem.Purge(), nil })
	default:
		log.Fatalf("unknown STORE=%q (use memory or postgres)", mode)
	}
	// C-RL: 3 scans per hour per client by default; RATE_LIMIT_PER_HOUR raises it for local testing.
	perHour := 3
	if v := os.Getenv("RATE_LIMIT_PER_HOUR"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			log.Fatalf("RATE_LIMIT_PER_HOUR must be a positive number, got %q", v)
		}
		perHour = n
	}
	limiter := ratelimit.New(perHour, time.Hour, 1)
	log.Printf("rate limit: %d scans per hour per client, 1 at a time", perHour)
	jobs := make(chan worker.Job, 32)

	// C-SSRF-DEV: allow scanning loopback targets (local test shops), only while the API itself is local.
	var policy ssrf.Policy
	if os.Getenv("ALLOW_LOOPBACK_TARGETS") == "1" {
		host, _, err := net.SplitHostPort(addr)
		if ip, perr := netip.ParseAddr(host); err != nil || (host != "localhost" && (perr != nil || !ip.IsLoopback())) {
			log.Fatalf("ALLOW_LOOPBACK_TARGETS=1 needs API_ADDR on a loopback address (e.g. 127.0.0.1:8080), got %q", addr)
		}
		policy.AllowLoopback = true
		log.Printf("WARNING: ALLOW_LOOPBACK_TARGETS=1: scans of 127.0.0.1/localhost are allowed (local testing only)")
	}

	fetcher := fetch.New(nil)
	fetcher.Policy = policy
	var (
		source    worker.PageSource
		annotator worker.Annotator // screenshots for the visual report (specs/11-report.md)
		printer   api.PDFPrinter   // PDF printed from the HTML report (ADR-7)
	)
	if path, err := crawler.FindBrowser(""); err == nil {
		log.Printf("browser crawler enabled (%s)", path)
		cr := &crawler.Crawler{
			ExecPath: path, NoSandbox: os.Getenv("CHROME_NO_SANDBOX") == "1",
			Validate: func(ctx context.Context, raw string) error { _, _, err := policy.Validate(ctx, raw, nil); return err },
		}
		source = worker.CrawlSource{
			Crawler: cr,
			Options: crawler.DefaultOptions(),
			Robots:  fetcher,
		}
		annotator = worker.CrawlAnnotator{Crawler: cr}
		printer = cr
	} else {
		log.Printf("WARNING: no browser found (%v): scanning the start page only, rules that need axe-core report 'not assessable'", err)
		source = worker.FetchSource{Client: fetcher}
	}

	allRules := []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()}
	ruleMeta := map[string]rules.Meta{}
	for _, r := range allRules {
		ruleMeta[r.Meta().ID] = r.Meta()
	}

	runner := &worker.Runner{
		Store:     store,
		Source:    source,
		Annotator: annotator,
		Rules:     allRules,
		Release:   limiter.Release,
		Timeout:   5 * time.Minute, // C-1
	}
	for i := 0; i < 2; i++ {
		go runner.Loop(ctx, jobs)
	}

	srv := &api.Server{
		Store: store, Limiter: limiter, Jobs: jobs, Salt: salt, SSRF: policy,
		TargetCooldown: 10 * time.Minute, // C-RL
		AllowOrigin:    os.Getenv("CORS_ORIGIN"),
		RuleMeta:       ruleMeta,
		PDF:            printer,
		TrustedProxies: trustedProxies(os.Getenv("TRUSTED_PROXIES")),
	}
	httpSrv := &http.Server{Addr: addr, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()

	log.Printf("api listening on %s", addr)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// purgeLoop deletes expired scans every minute (N4, ADR-6: 1-hour retention).
func purgeLoop(ctx context.Context, purge func() (int, error)) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		if n, err := purge(); err != nil {
			log.Printf("purge: %v", err)
		} else if n > 0 {
			log.Printf("purge: removed %d expired scans", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// trustedProxies parses TRUSTED_PROXIES, a comma-separated list of CIDRs or single addresses (C-RL-PROXY).
func trustedProxies(v string) []netip.Prefix {
	var out []netip.Prefix
	for _, s := range strings.Split(v, ",") {
		if s = strings.TrimSpace(s); s == "" {
			continue
		}
		p, err := netip.ParsePrefix(s)
		if err != nil {
			a, aerr := netip.ParseAddr(s)
			if aerr != nil {
				log.Fatalf("TRUSTED_PROXIES: %q is not a CIDR or address", s)
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		out = append(out, p)
	}
	if len(out) > 0 {
		log.Printf("trusting X-Forwarded-For from %v", out)
	}
	return out
}
