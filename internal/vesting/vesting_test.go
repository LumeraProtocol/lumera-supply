package vesting

import (
	"testing"
	"time"
)

func mustTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDelayedLocked(t *testing.T) {
	e := NewEngine()
	now := mustTime("2024-01-01T00:00:00Z")
	end := mustTime("2024-06-01T00:00:00Z")
	if got := e.DelayedLocked("1000", now, end); got != "1000" {
		t.Fatalf("before end expected 1000 got %s", got)
	}
	now = mustTime("2024-06-01T00:00:01Z")
	if got := e.DelayedLocked("1000", now, end); got != "0" {
		t.Fatalf("after end expected 0 got %s", got)
	}
}

func TestContinuousLocked(t *testing.T) {
	e := NewEngine()
	start := mustTime("2024-01-01T00:00:00Z")
	end := mustTime("2024-01-11T00:00:00Z") // 10 days
	now := start.Add(5 * 24 * time.Hour)
	got := e.ContinuousLocked("1000", now, start, end)
	if got != "500" {
		t.Fatalf("midway expected 500 got %s", got)
	}
	// before start
	now = start.Add(-1 * time.Second)
	if got := e.ContinuousLocked("1000", now, start, end); got != "1000" {
		t.Fatalf("before start expected 1000 got %s", got)
	}
	// after end
	now = end.Add(time.Second)
	if got := e.ContinuousLocked("1000", now, start, end); got != "0" {
		t.Fatalf("after end expected 0 got %s", got)
	}
}

func TestPeriodicLocked(t *testing.T) {
	e := NewEngine()
	base := mustTime("2024-01-01T00:00:00Z")
	periods := []Period{
		{End: base.Add(24 * time.Hour), Amount: "100"},
		{End: base.Add(2 * 24 * time.Hour), Amount: "100"},
		{End: base.Add(3 * 24 * time.Hour), Amount: "100"},
	}
	// before all
	if got := e.PeriodicLocked(periods, base); got != "300" {
		t.Fatalf("before all expected 300 got %s", got)
	}
	// after first
	if got := e.PeriodicLocked(periods, base.Add(24*time.Hour+time.Second)); got != "200" {
		t.Fatalf("after first expected 200 got %s", got)
	}
	// after all
	if got := e.PeriodicLocked(periods, base.Add(4*24*time.Hour)); got != "0" {
		t.Fatalf("after all expected 0 got %s", got)
	}
}

func TestClawbackLocked(t *testing.T) {
	e := NewEngine()
	start := mustTime("2024-01-01T00:00:00Z")
	cliff := mustTime("2024-02-01T00:00:00Z")
	end := mustTime("2024-03-01T00:00:00Z")
	// before cliff
	if got := e.ClawbackLocked("900", start.Add(24*time.Hour), start, cliff, end); got != "900" {
		t.Fatalf("before cliff expected 900 got %s", got)
	}
	// after cliff midway to end: locked ~ 50%
	now := start.Add(45 * 24 * time.Hour) // mid between cliff (31d) and end (59d)
	got := e.ClawbackLocked("900", now, start, cliff, end)
	if got == "900" || got == "0" {
		t.Fatalf("expected partial lock got %s", got)
	}
}

func TestPermanentLocked(t *testing.T) {
	e := NewEngine()
	if got := e.PermanentLocked("123456"); got != "123456" {
		t.Fatalf("expected 123456 got %s", got)
	}
}
