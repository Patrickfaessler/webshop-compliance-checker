// Command api serves the REST API described in specs/08-api.yaml and, in this interim setup,
// runs the scan worker in the same process (it moves to cmd/worker once PostgreSQL + a job queue exist).
package main

import (
	"context"
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	var store scan.Store
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		pg, err := pgstore.Open(openCtx, dbURL)
		cancel()
		if err != nil {
			log.Fatalf("database: %v", err)
		}
		defer pg.Close()
		store = pg
		log.Printf("database: PostgreSQL (migrations applied)")
		go purgeLoop(ctx, pg)
	} else {
		store = scan.NewMemoryStore()
		log.Printf("database: none (DATABASE_URL not set), scans are kept in memory and lost on restart")
	}
	limiter := ratelimit.New(3, time.Hour, 1)
	jobs := make(chan worker.Job, 32)

	fetcher := fetch.New(nil)
	var source worker.PageSource
	if path, err := crawler.FindBrowser(""); err == nil {
		log.Printf("browser crawler enabled (%s)", path)
		source = worker.CrawlSource{
			Crawler: &crawler.Crawler{ExecPath: path, NoSandbox: os.Getenv("CHROME_NO_SANDBOX") == "1"},
			Options: crawler.DefaultOptions(),
			Robots:  fetcher,
		}
	} else {
		log.Printf("WARNING: no browser found (%v): scanning the start page only, rules that need axe-core report 'not assessable'", err)
		source = worker.FetchSource{Client: fetcher}
	}

	runner := &worker.Runner{
		Store:   store,
		Source:  source,
		Rules:   []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()},
		Release: limiter.Release,
		Timeout: 5 * time.Minute, // C-1
	}
	for i := 0; i < 2; i++ {
		go runner.Loop(ctx, jobs)
	}

	srv := &api.Server{
		Store: store, Limiter: limiter, Jobs: jobs, Salt: salt,
		HostCooldown: 10 * time.Minute, // C-RL
		AllowOrigin:  os.Getenv("CORS_ORIGIN"),
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

// purgeLoop deletes expired scans every hour (N4: 30-day retention).
func purgeLoop(ctx context.Context, pg *pgstore.Store) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n, err := pg.Purge(ctx); err != nil {
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
