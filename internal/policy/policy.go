package policy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

// Policy defines cohorts, module accounts, and IBC channels to consider non-circulating.
// Only cohorts present in this policy are treated as non-circulating; user-created
// vesting accounts are considered circulating by default.
type Policy struct {
	// MaxSupply, if provided, is the protocol maximum supply for the denom.
	MaxSupply *string `json:"max_supply"`

	// ModuleAccounts are module account addresses or names to treat as non-circulating.
	ModuleAccounts []string `json:"module_accounts"`

	// IBC channels to include when computing escrow totals.
	IBCChannels []string `json:"ibc_channels"`

	// DisclosedLockups are foundation or team addresses with known lockups.
	DisclosedLockups []Cohort `json:"disclosed_lockups"`
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
	// Optional fields are fine; ensure names present in disclosed lockups
	for i, c := range p.DisclosedLockups {
		if c.Name == "" {
			return fmt.Errorf("disclosed_lockups[%d] missing name", i)
		}
	}
	return nil
}
