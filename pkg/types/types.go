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

// AddressItem represents per-address details for cohorts that require per-address reporting
// (e.g., foundation_genesis, claim_delayed, supernode_bootstraps).
// EndDate uses RFC3339 when applicable; for permanent locks, use "forever".
type AddressItem struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
	EndDate string `json:"end_date,omitempty"`
}

type CohortEntry struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
	// Address is used for single-address cohorts (e.g., module accounts).
	Address string `json:"address,omitempty"`
	// Items is used to list per-address details for cohorts.
	Items []AddressItem `json:"items,omitempty"`
	// Amount is the total amount for the cohort (sum of items when present).
	Amount string `json:"amount"`
}
