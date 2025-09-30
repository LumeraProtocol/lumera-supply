package lcd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type claimItem struct {
	OldAddress  string `json:"oldAddress"`
	Balance     []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"balance"`
	Claimed     bool   `json:"claimed"`
	ClaimTime   string `json:"claimTime"`
	DestAddress string `json:"destAddress"`
	VestedTier  int    `json:"vestedTier"`
}

type claimResp struct {
	Claims     []claimItem `json:"claims"`
	Pagination struct {
		NextKey interface{} `json:"next_key"`
		Total   string      `json:"total"`
	} `json:"pagination"`
}

func TestClaimListClaimed_NewShape(t *testing.T) {
	// Prepare response matching the issue description
	resp := claimResp{
		Claims: []claimItem{{
			OldAddress:  "Ptka6xgtFNymSsWYoPQM52p39TYmeMTniXz",
			Balance:     []struct{ Denom string `json:"denom"`; Amount string `json:"amount"` }{{Denom: "ulume", Amount: "12345"}},
			Claimed:     true,
			ClaimTime:   "1757782016",
			DestAddress: "lumera1le9lc6r8zjts72mj4cswg4y4nggsq094kv2yze",
			VestedTier:  1,
		}},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/LumeraProtocol/lumera/claim/list_claimed/1" {
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, ts.Client())

	recs, err := client.ClaimListClaimed(1, "ulume")
	if err != nil {
		t.Fatalf("ClaimListClaimed error: %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("expected 1 record got %d", len(recs))
	}
	if recs[0].Address != "lumera1le9lc6r8zjts72mj4cswg4y4nggsq094kv2yze" {
		t.Fatalf("unexpected address: %s", recs[0].Address)
	}
	if recs[0].Time == nil {
		t.Fatalf("expected non-nil claim time")
	}
	if recs[0].Amount != "12345" {
		t.Fatalf("unexpected amount: %s", recs[0].Amount)
	}
	want := time.Unix(1757782016, 0).UTC()
	if !recs[0].Time.Equal(want) {
		t.Fatalf("unexpected time: got %s want %s", recs[0].Time, want)
	}
}
