package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lumera-labs/lumera-supply/pkg/cache"
	"github.com/lumera-labs/lumera-supply/pkg/httpserver"
	"github.com/lumera-labs/lumera-supply/pkg/lcd"
	"github.com/lumera-labs/lumera-supply/pkg/policy"
	"github.com/lumera-labs/lumera-supply/pkg/supply"
)

var (
	GitTag    = "dev"
	GitCommit = "unknown"
)

func main() {
	var (
		addr       = flag.String("addr", getEnv("LUMERA_HTTP_ADDR", ":8080"), "HTTP listen address")
		lcdURL     = flag.String("lcd", getEnv("LUMERA_LCD_URL", "http://localhost:1317"), "Cosmos LCD base URL")
		policyPath = flag.String("policy", getEnv("LUMERA_POLICY_PATH", "policy.json"), "Path to policy JSON file")
		defaultDen = flag.String("denom", getEnv("LUMERA_DEFAULT_DENOM", "ulume"), "Default base denom")
	)
	flag.Parse()

	pol, err := policy.Load(*policyPath)
	if err != nil {
		log.Printf("policy load warning: %v (service will start but /circulating may be incomplete)", err)
	}

	client := lcd.NewClient(*lcdURL, &http.Client{Timeout: 5 * time.Second})

	// Supply computer
	computer := supply.NewComputer(client, pol)

	// Snapshot cache with refresher
	c := cache.NewSnapshotCache(computer, cache.Options{TTL: 60 * time.Second})
	go c.RunRefresher(*defaultDen)

	srv := httpserver.New(httpserver.Config{
		Cache:        c,
		Computer:     computer,
		DefaultDenom: *defaultDen,
		RatePerMin:   60,
		Burst:        120,
		GitTag:       GitTag,
		GitCommit:    GitCommit,
	})

	log.Printf("Lumera Supply API listening on %s (lcd=%s denom=%s)", *addr, *lcdURL, *defaultDen)
	log.Printf("Git tag: %s, Git commit: %s", GitTag, GitCommit)
	if err := http.ListenAndServe(*addr, srv.Mux()); err != nil {
		log.Fatal(err)
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
