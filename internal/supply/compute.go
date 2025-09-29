package supply

import (
	"crypto/sha1"
	"encoding/hex"
	"log"
	"math/big"
	"time"

	"github.com/lumera-labs/lumera-supply/internal/lcd"
	"github.com/lumera-labs/lumera-supply/internal/policy"
	"github.com/lumera-labs/lumera-supply/internal/types"
)

type Computer struct {
	lcd    *lcd.Client
	policy *policy.Policy
}

func NewComputer(l *lcd.Client, p *policy.Policy) *Computer {
	return &Computer{lcd: l, policy: p}
}

// ComputeSnapshot fetches on-chain data and computes a snapshot at latest height.
func (c *Computer) ComputeSnapshot(denom string) (*types.SupplySnapshot, error) {
	height, t, err := c.lcd.LatestHeight()
	if err != nil {
		return nil, err
	}
	total, err := c.lcd.TotalSupplyByDenom(denom)
	if err != nil {
		return nil, err
	}

	var breakdown types.NonCircBreakdown
	// Cohort: IBC escrow total
	if esc, err := c.lcd.IBCTotalEscrow(denom); err == nil {
		breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
			Name:   "ibc_escrow",
			Reason: "ICS20 transfer escrows",
			Amount: esc,
		})
	} else {
		log.Printf("warn: ibc escrow fetch failed: %v", err)
	}

	// Cohort: module accounts balances
	if c.policy != nil {
		for _, addr := range c.policy.ModuleAccounts {
			amt, err := c.lcd.BalanceByDenom(addr, denom)
			if err != nil {
				log.Printf("warn: module acct balance %s: %v", addr, err)
				continue
			}
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:      "module_account",
				Reason:    "protocol-controlled module account",
				Addresses: []string{addr},
				Amount:    amt,
			})
		}
		// Disclosed lockups
		for _, cohort := range c.policy.DisclosedLockups {
			sum := big.NewInt(0)
			for _, addr := range cohort.Addresses {
				amt, err := c.lcd.BalanceByDenom(addr, denom)
				if err != nil {
					log.Printf("warn: disclosed lockup balance %s: %v", addr, err)
					continue
				}
				v, _ := new(big.Int).SetString(amt, 10)
				sum.Add(sum, v)
			}
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:      cohort.Name,
				Reason:    cohort.Reason,
				Addresses: cohort.Addresses,
				Amount:    sum.String(),
			})
		}
	}

	// Sum non-circ
	sum := big.NewInt(0)
	for _, e := range breakdown.Cohorts {
		v, _ := new(big.Int).SetString(e.Amount, 10)
		sum.Add(sum, v)
	}
	breakdown.Sum = sum.String()

	// Circulating = total - non_circ
	T, _ := new(big.Int).SetString(total, 10)
	circ := new(big.Int).Sub(T, sum)
	if circ.Sign() < 0 {
		circ.SetInt64(0)
	}

	etag := computeETag(height, denom, total, circ.String(), breakdown.Sum)

	var max *string
	if c.policy != nil && c.policy.MaxSupply != nil {
		max = c.policy.MaxSupply
	}

	return &types.SupplySnapshot{
		Denom:         denom,
		Height:        height,
		UpdatedAt:     t.UTC(),
		ETag:          etag,
		Total:         total,
		Circulating:   circ.String(),
		Max:           max,
		NonCirculating: breakdown,
	}, nil
}

func computeETag(height int64, denom, total, circ, non string) string {
	h := sha1.New()
	h.Write([]byte(denom))
	h.Write([]byte{0})
	h.Write([]byte(total))
	h.Write([]byte{0})
	h.Write([]byte(circ))
	h.Write([]byte{0})
	h.Write([]byte(non))
	h.Write([]byte{0})
	h.Write([]byte(time.Unix(height, 0).UTC().Format(time.RFC3339)))
	return hex.EncodeToString(h.Sum(nil))
}
