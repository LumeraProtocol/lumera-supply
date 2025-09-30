package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// Policy defines cohorts, module accounts, and IBC channels to consider non-circulating.
// Only cohorts present in this policy are treated as non-circulating; user-created
// vesting accounts are considered circulating by default.
type Policy struct {
	// MaxSupply, if provided, is the protocol maximum supply for the denom.
	MaxSupply *string `json:"max_supply"`

	// ModuleAccounts are module account names (preferred) or addresses to treat as non-circulating.
	// If an entry looks like a bech32 address (e.g., starts with "lumera1"), it will be treated as an address
	// for backward compatibility with older policies and tests.
	ModuleAccounts []string `json:"module_accounts"`

	// New nested disclosed lockups structure.
	Disclosed DisclosedLockups `json:"disclosed_lockups"`

	// Backward-compatibility: older flat cohorts used in tests (not populated from JSON).
	DisclosedLockups []Cohort `json:"-"`
}

type DisclosedLockups struct {
	FoundationGenesis   []FoundationEntry `json:"foundation_genesis"`
	SupernodeBootstraps []SupernodeEntry  `json:"supernode_bootstraps"`
	Timelocks           []json.RawMessage `json:"timelocks"`
	PartnersLockups     []json.RawMessage `json:"partners_lockups"`
}

type FoundationEntry struct {
	Name    string `json:"name"`
	Amount  string `json:"amount,omitempty"`
	Reason  string `json:"reason,omitempty"`
	Address string `json:"address"`
	Custody string `json:"custody,omitempty"`
}

type SupernodeEntry struct {
	Name           string     `json:"name"`
	Address        string     `json:"address"`
	Permanent      bool       `json:"permanent,omitempty"`
	DurationMonths *int       `json:"duration_months,omitempty"`
	StartTime      *time.Time `json:"start_time,omitempty"`
	EndTime        *time.Time `json:"end_time,omitempty"`
}

type Cohort struct {
	Name      string   `json:"name"`
	Reason    string   `json:"reason"`
	Addresses []string `json:"addresses"`
}

func Load(path string) (*Policy, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var p Policy
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Policy) Validate() error {
	if p == nil {
		return errors.New("nil policy")
	}
	// Validate nested disclosed lockups when present
	for i, e := range p.Disclosed.FoundationGenesis {
		if e.Name == "" {
			return fmt.Errorf("disclosed_lockups.foundation_genesis[%d] missing name", i)
		}
		if e.Address == "" {
			return fmt.Errorf("disclosed_lockups.foundation_genesis[%d] missing address", i)
		}
	}
	for i, e := range p.Disclosed.SupernodeBootstraps {
		if e.Address == "" {
			return fmt.Errorf("disclosed_lockups.supernode_bootstraps[%d] missing address", i)
		}
	}
	// Back-compat: ensure names present in flat disclosed lockups if used programmatically
	for i, c := range p.DisclosedLockups {
		if c.Name == "" {
			return fmt.Errorf("disclosed_lockups(flat)[%d] missing name", i)
		}
	}
	return nil
}
