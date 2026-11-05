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
	"compliancechecker/internal/crawler"
	"compliancechecker/internal/fetch"
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

	store := scan.NewMemoryStore()
	limiter := ratelimit.New(3, time.Hour, 1)
	jobs := make(chan worker.Job, 32)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

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
