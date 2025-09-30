package httpserver

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/lumera-labs/lumera-supply/pkg/cache"
	"github.com/lumera-labs/lumera-supply/pkg/ratelimit"
	"github.com/lumera-labs/lumera-supply/pkg/supply"
	"github.com/lumera-labs/lumera-supply/pkg/types"
)

type Config struct {
	Cache        *cache.SnapshotCache
	Computer     *supply.Computer
	DefaultDenom string
	RatePerMin   int
	Burst        int
}

type Server struct {
	cfg     Config
	mux     *http.ServeMux
	limiter *ratelimit.Limiter
}

func New(cfg Config) *Server {
	lim := ratelimit.New(cfg.RatePerMin, cfg.Burst)
	s := &Server{cfg: cfg, mux: http.NewServeMux(), limiter: lim}
	// public endpoints
	s.mux.HandleFunc("/healthz", s.healthz)
	s.mux.HandleFunc("/total", s.wrap(s.handleTotal))
	s.mux.HandleFunc("/circulating", s.wrap(s.handleCirculating))
	s.mux.HandleFunc("/non_circulating", s.wrap(s.handleNonCirc))
	s.mux.HandleFunc("/max", s.wrap(s.handleMax))
	return s
}

func (s *Server) Mux() *http.ServeMux { return s.mux }

func (s *Server) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow(r) {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=30")
		next(w, r)
	}
}

func (s *Server) parseDenom(r *http.Request) (string, bool) {
	denom := r.URL.Query().Get("denom")
	if denom == "" {
		denom = s.cfg.DefaultDenom
	}
	if denom == "" || len(denom) > 64 {
		return "", false
	}
	return denom, true
}

func (s *Server) snapshot(r *http.Request, denom string) (*response, int, error) {
	ifNone := r.Header.Get("If-None-Match")
	if snap, fresh := s.cfg.Cache.Get(); snap != nil && fresh && ifNone == snap.ETag && snap.Denom == denom {
		return nil, http.StatusNotModified, nil
	}
	// Use cache if fresh, else recompute and refresh
	if snap, fresh := s.cfg.Cache.Get(); snap != nil && fresh && snap.Denom == denom {
		return &response{snap: snap}, http.StatusOK, nil
	}
	snap, err := s.cfg.Cache.Update(denom)
	if err != nil {
		return nil, 0, err
	}
	return &response{snap: snap}, http.StatusOK, nil
}

type response struct {
	snap *types.SupplySnapshot // raw snapshot; projected per endpoint
}

type typesSnapshot struct {
	Denom       string    `json:"denom"`
	Height      int64     `json:"height"`
	UpdatedAt   time.Time `json:"updated_at"`
	ETag        string    `json:"etag"`
	Total       string    `json:"total"`
	Circulating string    `json:"circulating"`
	Max         *string   `json:"max"`
	NonCirc     nonCirc   `json:"non_circulating"`
}

type nonCirc struct {
	Sum     string        `json:"sum"`
	Cohorts []cohortEntry `json:"cohorts"`
}

type addressItem struct {
	Address string `json:"address"`
	Amount  string `json:"amount"`
	EndDate string `json:"end_date,omitempty"`
}

type cohortEntry struct {
	Name    string        `json:"name"`
	Reason  string        `json:"reason"`
	Address string        `json:"address,omitempty"`
	Items   []addressItem `json:"items,omitempty"`
	Amount  string        `json:"amount"`
}

// projection helper
func toTypesSnapshot(s *types.SupplySnapshot) *typesSnapshot {
	coh := make([]cohortEntry, 0, len(s.NonCirculating.Cohorts))
	for _, c := range s.NonCirculating.Cohorts {
		// map items
		items := make([]addressItem, 0, len(c.Items))
		for _, it := range c.Items {
			items = append(items, addressItem{Address: it.Address, Amount: it.Amount, EndDate: it.EndDate})
		}
		coh = append(coh, cohortEntry{Name: c.Name, Reason: c.Reason, Address: c.Address, Items: items, Amount: c.Amount})
	}
	return &typesSnapshot{
		Denom:       s.Denom,
		Height:      s.Height,
		UpdatedAt:   s.UpdatedAt,
		ETag:        s.ETag,
		Total:       s.Total,
		Circulating: s.Circulating,
		Max:         s.Max,
		NonCirc:     nonCirc{Sum: s.NonCirculating.Sum, Cohorts: coh},
	}
}

func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, snap *types.SupplySnapshot, project func(*typesSnapshot) any) {
	w.Header().Set("ETag", snap.ETag)
	w.Header().Set("X-Block-Height", itoa64(snap.Height))
	w.Header().Set("X-Updated-At", snap.UpdatedAt.Format(time.RFC3339))
	payload := project(toTypesSnapshot(snap))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func (s *Server) handleTotal(w http.ResponseWriter, r *http.Request) {
	denom, ok := s.parseDenom(r)
	if !ok {
		http.Error(w, "invalid denom", http.StatusBadRequest)
		return
	}
	resp, status, err := s.snapshot(r, denom)
	if err != nil {
		log.Printf("/total error: %v", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	if status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}
	snap := resp.snap
	// output minimal fields
	srv := toTypesSnapshot(snap)
	out := struct {
		Denom          string    `json:"denom"`
		Decimals       int       `json:"decimals"`
		Height         int64     `json:"height"`
		UpdatedAt      time.Time `json:"updated_at"`
		ETag           string    `json:"etag"`
		Total          string    `json:"total"`
		Circulating    string    `json:"circulating"`
		NonCirculating string    `json:"non_circulating"`
		Max            *string   `json:"max"`
	}{srv.Denom, 6, srv.Height, srv.UpdatedAt, srv.ETag, srv.Total, srv.Circulating, srv.NonCirc.Sum, srv.Max}
	w.Header().Set("ETag", srv.ETag)
	w.Header().Set("X-Block-Height", itoa64(srv.Height))
	w.Header().Set("X-Updated-At", srv.UpdatedAt.Format(time.RFC3339))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func (s *Server) handleMax(w http.ResponseWriter, r *http.Request) {
	denom, ok := s.parseDenom(r)
	if !ok {
		http.Error(w, "invalid denom", http.StatusBadRequest)
		return
	}
	resp, status, err := s.snapshot(r, denom)
	if err != nil {
		log.Printf("/max error: %v", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	if status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}
	snap := resp.snap
	w.Header().Set("ETag", snap.ETag)
	w.Header().Set("X-Block-Height", itoa64(snap.Height))
	w.Header().Set("X-Updated-At", snap.UpdatedAt.Format(time.RFC3339))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		Denom     string    `json:"denom"`
		Decimals  int       `json:"decimals"`
		Height    int64     `json:"height"`
		UpdatedAt time.Time `json:"updated_at"`
		ETag      string    `json:"etag"`
		Max       *string   `json:"max"`
	}{snap.Denom, 6, snap.Height, snap.UpdatedAt, snap.ETag, snap.Max})
}

func (s *Server) handleCirculating(w http.ResponseWriter, r *http.Request) {
	denom, ok := s.parseDenom(r)
	if !ok {
		http.Error(w, "invalid denom", http.StatusBadRequest)
		return
	}
	resp, status, err := s.snapshot(r, denom)
	if err != nil {
		log.Printf("/circulating error: %v", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	if status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}
	snap := resp.snap
	srv := toTypesSnapshot(snap)
	out := struct {
		Denom          string    `json:"denom"`
		Decimals       int       `json:"decimals"`
		Height         int64     `json:"height"`
		UpdatedAt      time.Time `json:"updated_at"`
		ETag           string    `json:"etag"`
		Circulating    string    `json:"circulating"`
		NonCirculating string    `json:"non_circulating"`
	}{srv.Denom, 6, srv.Height, srv.UpdatedAt, srv.ETag, srv.Circulating, srv.NonCirc.Sum}
	w.Header().Set("ETag", srv.ETag)
	w.Header().Set("X-Block-Height", itoa64(srv.Height))
	w.Header().Set("X-Updated-At", srv.UpdatedAt.Format(time.RFC3339))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func (s *Server) handleNonCirc(w http.ResponseWriter, r *http.Request) {
	denom, ok := s.parseDenom(r)
	if !ok {
		http.Error(w, "invalid denom", http.StatusBadRequest)
		return
	}
	resp, status, err := s.snapshot(r, denom)
	if err != nil {
		log.Printf("/non_circulating error: %v", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	if status == http.StatusNotModified {
		w.WriteHeader(status)
		return
	}
	snap := resp.snap
	srv := toTypesSnapshot(snap)
	out := struct {
		Denom     string    `json:"denom"`
		Decimals  int       `json:"decimals"`
		Height    int64     `json:"height"`
		UpdatedAt time.Time `json:"updated_at"`
		ETag      string    `json:"etag"`
		Breakdown nonCirc   `json:"non_circulating"`
	}{srv.Denom, 6, srv.Height, srv.UpdatedAt, srv.ETag, srv.NonCirc}
	w.Header().Set("ETag", srv.ETag)
	w.Header().Set("X-Block-Height", itoa64(srv.Height))
	w.Header().Set("X-Updated-At", srv.UpdatedAt.Format(time.RFC3339))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	_ = enc.Encode(struct {
		Status string `json:"status"`
		Time   string `json:"time"`
	}{"ok", time.Now().UTC().Format(time.RFC3339)})
}

func itoa64(n int64) string {
	// fast int64 to string without strconv import
	return (&struct{ s string }{s: func() string { return fmtInt(n) }()}).s
}

func fmtInt(n int64) string {
	// simple base-10
	if n == 0 {
		return "0"
	}
	sign := false
	if n < 0 {
		sign = true
		n = -n
	}
	var a [20]byte
	i := len(a)
	for n > 0 {
		i--
		a[i] = byte('0' + n%10)
		n /= 10
	}
	if sign {
		i--
		a[i] = '-'
	}
	return string(a[i:])
}
