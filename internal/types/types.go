package types

import "time"

// SupplySnapshot is an atomic snapshot of supply-related figures for a given block height.
// All values are in base denom units as strings to avoid float rounding; use integers in atoms.
type SupplySnapshot struct {
	Denom          string           `json:"denom"`
	Height         int64            `json:"height"`
	UpdatedAt      time.Time        `json:"updated_at"`
	ETag           string           `json:"etag"`
	Total          string           `json:"total"`
	Circulating    string           `json:"circulating"`
	Max            *string          `json:"max"`
	NonCirculating NonCircBreakdown `json:"non_circulating"`
}

type NonCircBreakdown struct {
	Sum     string        `json:"sum"`
	Cohorts []CohortEntry `json:"cohorts"`
}

type CohortEntry struct {
	Name      string   `json:"name"`
	Reason    string   `json:"reason"`
	Addresses []string `json:"addresses,omitempty"`
	Amount    string   `json:"amount"`
}
