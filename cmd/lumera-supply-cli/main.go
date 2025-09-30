package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lumera-labs/lumera-supply/pkg/lcd"
	"github.com/lumera-labs/lumera-supply/pkg/policy"
	"github.com/lumera-labs/lumera-supply/pkg/supply"
	"github.com/lumera-labs/lumera-supply/pkg/types"
)

func main() {
	var (
		lcdURL     = flag.String("lcd", getEnv("LUMERA_LCD_URL", "http://localhost:1317"), "Cosmos LCD base URL")
		policyPath = flag.String("policy", getEnv("LUMERA_POLICY_PATH", "policy.json"), "Path to policy JSON file")
		denom      = flag.String("denom", getEnv("LUMERA_DEFAULT_DENOM", "ulume"), "Base denom (e.g., ulume)")
		pretty     = flag.Bool("pretty", true, "Pretty-print JSON output")
	)
	flag.Parse()

	// Load policy (warn-only if missing)
	pol, err := policy.Load(*policyPath)
	if err != nil {
		log.Printf("policy load warning: %v (continuing without policy)", err)
	}

	client := lcd.NewClient(*lcdURL, &http.Client{Timeout: 8 * time.Second})
	comp := supply.NewComputer(client, pol)

	snap, err := comp.ComputeSnapshot(*denom)
	if err != nil {
		log.Fatalf("compute snapshot failed: %v", err)
	}

	out := projectCLI(snap)
	enc := json.NewEncoder(os.Stdout)
	if *pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(out); err != nil {
		log.Fatalf("encode failed: %v", err)
	}
}

func projectCLI(s *types.SupplySnapshot) any {
	// Shape: match API semantics; include totals and full non_circulating breakdown for auditing
	type addressItem struct {
		Address string `json:"address"`
		Amount  string `json:"amount"`
		EndDate string `json:"end_date,omitempty"`
	}
	type cohortEntry struct {
		Name    string        `json:"name"`
		Reason  string        `json:"reason"`
		Address string        `json:"address,omitempty"`
		Items   []addressItem `json:"items,omitempty"`
		Amount  string        `json:"amount"`
	}
	type nonCirc struct {
		Sum     string        `json:"sum"`
		Cohorts []cohortEntry `json:"cohorts"`
	}
	// map cohorts
	coh := make([]cohortEntry, 0, len(s.NonCirculating.Cohorts))
	for _, c := range s.NonCirculating.Cohorts {
		items := make([]addressItem, 0, len(c.Items))
		for _, it := range c.Items {
			items = append(items, addressItem{Address: it.Address, Amount: it.Amount, EndDate: it.EndDate})
		}
		coh = append(coh, cohortEntry{Name: c.Name, Reason: c.Reason, Address: c.Address, Items: items, Amount: c.Amount})
	}
	return struct {
		Denom          string    `json:"denom"`
		Decimals       int       `json:"decimals"`
		Height         int64     `json:"height"`
		UpdatedAt      time.Time `json:"updated_at"`
		ETag           string    `json:"etag"`
		Total          string    `json:"total"`
		Circulating    string    `json:"circulating"`
		NonCirculating nonCirc   `json:"non_circulating"`
		Max            *string   `json:"max"`
	}{
		Denom:          s.Denom,
		Decimals:       6,
		Height:         s.Height,
		UpdatedAt:      s.UpdatedAt,
		ETag:           s.ETag,
		Total:          s.Total,
		Circulating:    s.Circulating,
		NonCirculating: nonCirc{Sum: s.NonCirculating.Sum, Cohorts: coh},
		Max:            s.Max,
	}
}

func getEnv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
