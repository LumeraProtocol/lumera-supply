package vesting

import (
	"math/big"
	"time"
)

// Engine exposes methods to compute locked amount at a point in time for different vesting types.
// All amounts are strings of integer base units.
type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

// DelayedLocked - nothing unlocked until End; at End all unlocked.
func (e *Engine) DelayedLocked(total string, now, end time.Time) string {
	if !now.Before(end) {
		return "0"
	}
	return total
}

// ContinuousLocked - linear unlock from Start to End.
func (e *Engine) ContinuousLocked(total string, now, start, end time.Time) string {
	if !now.Before(start) && !now.Before(end) {
		return "0"
	}
	if now.Before(start) {
		return total
	}
	// ratio locked = 1 - progress
	locked := mulRatio(total, now.Sub(start), end.Sub(start))
	return locked
}

// Period - sum of periods; unlocks stepwise at each period end.
type Period struct {
	End    time.Time
	Amount string
}

func (e *Engine) PeriodicLocked(periods []Period, now time.Time) string {
	locked := big.NewInt(0)
	for _, p := range periods {
		if now.Before(p.End) {
			add(locked, p.Amount)
		}
	}
	return locked.String()
}

// ClawbackLocked (vesting with unvested amount subject to clawback until cliff):
// For simplicity: behaves like Continuous with a cliff; before cliff 100% locked.
func (e *Engine) ClawbackLocked(total string, now, start, cliff, end time.Time) string {
	if now.Before(cliff) {
		return total
	}
	return e.ContinuousLocked(total, now, start, end)
}

// PermanentLocked - never unlocks.
func (e *Engine) PermanentLocked(total string) string { return total }

// Helpers
func add(dst *big.Int, s string) {
	v, _ := new(big.Int).SetString(s, 10)
	dst.Add(dst, v)
}

func mulRatio(total string, num time.Duration, den time.Duration) string {
	if den <= 0 {
		return "0"
	}
	T, _ := new(big.Int).SetString(total, 10)
	n := big.NewInt(int64(num))
	d := big.NewInt(int64(den))
	// locked = T * (den - num) / den
	rem := new(big.Int).Sub(d, n)
	res := new(big.Int).Mul(T, rem)
	res.Quo(res, d)
	if res.Sign() < 0 {
		return "0"
	}
	return res.String()
}
