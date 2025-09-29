package supply

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lumera-labs/lumera-supply/internal/lcd"
	"github.com/lumera-labs/lumera-supply/internal/policy"
)

type latestBlockResponse struct {
	Block struct {
		Header struct {
			Height string    `json:"height"`
			Time   time.Time `json:"time"`
		} `json:"header"`
	} `json:"block"`
}

type amount struct {
	Denom  string `json:"denom,omitempty"`
	Amount string `json:"amount"`
}

func TestInvariantTotalEqualsCircPlusNonCirc(t *testing.T) {
	// Define balances
	const (
		total     = "1000000"
		ibcEscrow = "10000"
		modAddr   = "lumera1modulexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
		modBal    = "5000"
		lockAddr  = "lumera1foundationxxxxxxxxxxxxxxxxxxxxxxxxx"
		lockBal   = "15000"
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/cosmos/base/tendermint/v1beta1/blocks/latest":
			json.NewEncoder(w).Encode(latestBlockResponse{Block: struct{ Header struct{ Height string `json:"height"`; Time time.Time `json:"time"` } `json:"header"` }{Header: struct{ Height string `json:"height"`; Time time.Time `json:"time"` }{Height: "12345", Time: time.Now().UTC()}}})
		case r.URL.Path == "/cosmos/bank/v1beta1/supply/by_denom":
			json.NewEncoder(w).Encode(struct{ Amount amount `json:"amount"` }{Amount: amount{Denom: "ulume", Amount: total}})
		case r.URL.Path == "/ibc/apps/transfer/v1/denoms/ulume/total_escrow":
			json.NewEncoder(w).Encode(struct{ Amount amount `json:"amount"` }{Amount: amount{Amount: ibcEscrow}})
		case r.URL.Path == "/cosmos/bank/v1beta1/balances/"+modAddr+"/by_denom":
			json.NewEncoder(w).Encode(struct{ Balance amount `json:"balance"` }{Balance: amount{Amount: modBal}})
		case r.URL.Path == "/cosmos/bank/v1beta1/balances/"+lockAddr+"/by_denom":
			json.NewEncoder(w).Encode(struct{ Balance amount `json:"balance"` }{Balance: amount{Amount: lockBal}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	client := lcd.NewClient(ts.URL, ts.Client())
	pol := &policy.Policy{ModuleAccounts: []string{modAddr}, DisclosedLockups: []policy.Cohort{{Name: "foundation", Reason: "lockup", Addresses: []string{lockAddr}}}}
	comp := NewComputer(client, pol)

	snap, err := comp.ComputeSnapshot("ulume")
	if err != nil {
		t.Fatalf("compute snapshot error: %v", err)
	}

	if snap.Total == "" || snap.Circulating == "" || snap.NonCirculating.Sum == "" {
		t.Fatalf("missing fields in snapshot")
	}
	// Verify invariant: total = circ + non
	// Convert to big ints
	toInt := func(s string) int64 {
		var n int64
		_, _ = fmt.Sscan(s, &n)
		return n
	}
	T := toInt(snap.Total)
	C := toInt(snap.Circulating)
	N := toInt(snap.NonCirculating.Sum)
	if T != C+N {
		t.Fatalf("invariant failed: T=%d C=%d N=%d", T, C, N)
	}
	if snap.ETag == "" {
		t.Fatalf("etag missing")
	}
}
