package supply

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/lumera-labs/lumera-supply/pkg/lcd"
	"github.com/lumera-labs/lumera-supply/pkg/policy"
	"github.com/lumera-labs/lumera-supply/pkg/types"
	"github.com/lumera-labs/lumera-supply/pkg/vesting"
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

	ve := vesting.NewEngine()
	var breakdown types.NonCircBreakdown

	// Cohort: IBC escrow total (single call aggregates all transfer channels)
	if esc, err := c.lcd.IBCTotalEscrow(denom); err == nil {
		breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
			Name:   "ibc_escrow",
			Reason: "ICS20 transfer escrows",
			Amount: esc,
		})
	} else {
		log.Printf("warn: ibc escrow fetch failed: %v", err)
	}
	// Community pool (distribution module)
	if cp, err := c.lcd.CommunityPool(denom); err == nil {
		breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
			Name:   "community_pool",
			Reason: "distribution community pool",
			Amount: cp,
		})
	} else {
		log.Printf("warn: community pool fetch failed: %v", err)
	}

	if c.policy != nil {
		// Module accounts: accept names; report single address
		for _, accountName := range c.policy.ModuleAccounts {
			var accountAddress string
			if a, err := c.lcd.ModuleAddressByName(accountName); err == nil && a != "" {
				accountAddress = a
			} else {
				log.Printf("warn: module name %q resolution failed: %v", accountName, err)
				continue
			}
			amt, err := c.lcd.BalanceByDenom(accountAddress, denom)
			if err != nil {
				log.Printf("warn: module acct balance %s: %v", accountAddress, err)
				continue
			}
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:    "module:" + accountName,
				Reason:  "protocol-controlled module account",
				Address: accountAddress,
				Amount:  amt,
			})
		}

		// Foundation genesis: compute locked portion per address; include end_date
		if len(c.policy.Disclosed.FoundationGenesis) > 0 {
			items := make([]types.AddressItem, 0, len(c.policy.Disclosed.FoundationGenesis))
			totalLocked := big.NewInt(0)
			for _, e := range c.policy.Disclosed.FoundationGenesis {
				locked, end, _, err := c.lockedAndEndFromAuthAccount(e.Address, t, denom, ve)
				if err != nil {
					log.Printf("warn: foundation vesting compute for %s: %v", e.Address, err)
					continue
				}
				v, _ := new(big.Int).SetString(locked, 10)
				totalLocked.Add(totalLocked, v)
				items = append(items, types.AddressItem{Address: e.Address, Amount: locked, EndDate: end})
			}
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:   "foundation_genesis",
				Reason: "protocol/foundation vesting locked portion",
				Items:  items,
				Amount: totalLocked.String(),
			})
		}

		// Supernode bootstraps: from policy + on-chain; include per-address end_date (or forever)
		if len(c.policy.Disclosed.SupernodeBootstraps) > 0 {
			items := make([]types.AddressItem, 0, len(c.policy.Disclosed.SupernodeBootstraps))
			totalLocked := big.NewInt(0)
			for _, e := range c.policy.Disclosed.SupernodeBootstraps {
				locked, end, _, err := c.lockedAndEndFromAuthAccount(e.Address, t, denom, ve)
				if err != nil || locked == "0" {
					// Fallback to policy hints
					if e.Permanent {
						if bal, err2 := c.lcd.BalanceByDenom(e.Address, denom); err2 == nil {
							locked = bal
							end = "forever"
							err = nil
						}
					} else if e.DurationMonths != nil {
						start := e.StartTime
						if start == nil {
							start = &t
						}
						endTime := start.AddDate(0, *e.DurationMonths, 0)
						if bal, err2 := c.lcd.BalanceByDenom(e.Address, denom); err2 == nil {
							locked = ve.DelayedLocked(bal, t, endTime)
							end = endTime.UTC().Format(time.RFC3339)
							err = nil
						}
					}
				}
				v, _ := new(big.Int).SetString(locked, 10)
				totalLocked.Add(totalLocked, v)
				items = append(items, types.AddressItem{Address: e.Address, Amount: locked, EndDate: end})
			}
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:   "supernode_bootstraps",
				Reason: "protocol supernode bootstrap locks",
				Items:  items,
				Amount: totalLocked.String(),
			})
		}

		// Claimed accounts delayed locks (tiers 1..4): prefer on-chain vesting via AuthAccount; fallback to claim-record schedule; per-address
		claimedLocked := big.NewInt(0)
		items := make([]types.AddressItem, 0)
		for tier := 1; tier <= 4; tier++ {
			recs, err := c.lcd.ClaimListClaimed(tier, denom)
			if err != nil {
				log.Printf("warn: claim list tier %d: %v", tier, err)
				continue
			}
			months := tier * 6 // 1=>6m,2=>12m,3=>18m,4=>24m
			for _, r := range recs {
				if locked, end, _, err := c.lockedAndEndFromAuthAccount(r.Address, t, denom, ve); err == nil && locked != "" {
					v, _ := new(big.Int).SetString(locked, 10)
					claimedLocked.Add(claimedLocked, v)
					items = append(items, types.AddressItem{Address: r.Address, Amount: locked, EndDate: end})
					continue
				}
				// Fallback: delayed vesting from claim time
				start := t
				if r.Time != nil {
					start = *r.Time
				}
				endTime := start.AddDate(0, months, 0)
				amt := r.Amount
				if amt == "" { // fallback to on-chain balance if claim record lacks amount
					if bal, err := c.lcd.BalanceByDenom(r.Address, denom); err == nil {
						amt = bal
					}
				}
				if amt != "" {
					locked := ve.DelayedLocked(amt, t, endTime)
					v, _ := new(big.Int).SetString(locked, 10)
					claimedLocked.Add(claimedLocked, v)
					items = append(items, types.AddressItem{Address: r.Address, Amount: locked, EndDate: endTime.UTC().Format(time.RFC3339)})
				}
			}
		}
		if claimedLocked.Sign() > 0 || len(items) > 0 {
			breakdown.Cohorts = append(breakdown.Cohorts, types.CohortEntry{
				Name:   "claim_delayed",
				Reason: "claim module delayed locks (6/12/18/24m) with on-chain vesting preference",
				Items:  items,
				Amount: claimedLocked.String(),
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

	var maxSupply *string
	if c.policy != nil && c.policy.MaxSupply != nil {
		maxSupply = c.policy.MaxSupply
	}

	return &types.SupplySnapshot{
		Denom:          denom,
		Height:         height,
		UpdatedAt:      t.UTC(),
		ETag:           etag,
		Total:          total,
		Circulating:    circ.String(),
		Max:            maxSupply,
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

// lockedFromAuthAccount computes the locked amount for a vesting account based on its on-chain account JSON.
func (c *Computer) lockedFromAuthAccount(address string, now time.Time, denom string, ve *vesting.Engine) (string, error) {
	locked, _, _, err := c.lockedAndEndFromAuthAccount(address, now, denom, ve)
	return locked, err
}

// lockedAndEndFromAuthAccount computes the locked amount and end date (if any) for a vesting account based on its on-chain account JSON.
// Returns (locked, endDate, accountType, error). endDate is RFC3339, or "forever" for permanent locks, or empty if not applicable.
func (c *Computer) lockedAndEndFromAuthAccount(address string, now time.Time, denom string, ve *vesting.Engine) (string, string, string, error) {
	acctRaw, typ, err := c.lcd.AuthAccount(address)
	if err != nil {
		return "", "", "", err
	}
	// Generic struct covering common vesting account fields
	var v struct {
		BaseVestingAccount struct {
			OriginalVesting []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"original_vesting"`
			EndTime string `json:"end_time"`
		} `json:"base_vesting_account"`
		StartTime      string `json:"start_time"`
		VestingPeriods []struct {
			Length string `json:"length"`
			Amount []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"amount"`
		} `json:"vesting_periods"`
	}
	if err := json.Unmarshal(acctRaw, &v); err != nil {
		return "", "", "", err
	}
	ov := "0"
	for _, c := range v.BaseVestingAccount.OriginalVesting {
		if c.Denom == denom {
			ov = c.Amount
			break
		}
	}
	// If no vesting info, nothing locked
	if ov == "0" {
		return "0", "", typ, nil
	}
	// Helpers to parse times (seconds since epoch in strings)
	parseTS := func(s string) time.Time {
		if s == "" {
			return time.Time{}
		}
		var sec int64
		_, _ = fmt.Sscan(s, &sec)
		return time.Unix(sec, 0).UTC()
	}
	start := parseTS(v.StartTime)
	end := parseTS(v.BaseVestingAccount.EndTime)

	switch {
	case strings.Contains(typ, "PermanentLockedAccount"):
		return ve.PermanentLocked(ov), "forever", typ, nil
	case strings.Contains(typ, "DelayedVestingAccount"):
		endStr := ""
		if !end.IsZero() {
			endStr = end.Format(time.RFC3339)
		}
		return ve.DelayedLocked(ov, now, end), endStr, typ, nil
	case strings.Contains(typ, "ContinuousVestingAccount"):
		endStr := ""
		if !end.IsZero() {
			endStr = end.Format(time.RFC3339)
		}
		return ve.ContinuousLocked(ov, now, start, end), endStr, typ, nil
	case strings.Contains(typ, "PeriodicVestingAccount") || strings.Contains(typ, "ClawbackVestingAccount"):
		// Build periods timeline and remember the last end time
		elapsed := time.Duration(0)
		periods := make([]vesting.Period, 0, len(v.VestingPeriods))
		for _, p := range v.VestingPeriods {
			var durSec int64
			_, _ = fmt.Sscan(p.Length, &durSec)
			elapsed += time.Second * time.Duration(durSec)
			amount := "0"
			for _, a := range p.Amount {
				if a.Denom == denom {
					amount = a.Amount
					break
				}
			}
			periods = append(periods, vesting.Period{End: start.Add(elapsed), Amount: amount})
		}
		locked := ve.PeriodicLocked(periods, now)
		var endStr string
		if len(periods) > 0 {
			endStr = periods[len(periods)-1].End.UTC().Format(time.RFC3339)
		}
		return locked, endStr, typ, nil
	default:
		// Unknown type: assume not vesting
		return "0", "", typ, nil
	}
}
