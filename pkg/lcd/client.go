package lcd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	base   string
	client *http.Client
}

// decToIntString truncates a decimal string to its integer part (no rounding).
func decToIntString(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			if i == 0 {
				return "0"
			}
			return s[:i]
		}
	}
	return s
}

func NewClient(base string, httpClient *http.Client) *Client {
	return &Client{base: strings.TrimRight(base, "/"), client: httpClient}
}

// LatestHeight returns the latest block height and time from LCD.
func (c *Client) LatestHeight() (int64, time.Time, error) {
	u := c.base + "/cosmos/base/tendermint/v1beta1/blocks/latest"
	resp, err := c.client.Get(u)
	if err != nil {
		return 0, time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return 0, time.Time{}, fmt.Errorf("lcd latest block: %s", string(b))
	}
	var out struct {
		Block struct {
			Header struct {
				Height string    `json:"height"`
				Time   time.Time `json:"time"`
			} `json:"header"`
		} `json:"block"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, time.Time{}, err
	}
	h, err := parseInt(out.Block.Header.Height)
	if err != nil {
		return 0, time.Time{}, err
	}
	return h, out.Block.Header.Time, nil
}

// TotalSupplyByDenom returns the total on-chain supply for a denom.
func (c *Client) TotalSupplyByDenom(denom string) (string, error) {
	u := c.base + "/cosmos/bank/v1beta1/supply/by_denom?denom=" + url.QueryEscape(denom)
	resp, err := c.client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("lcd supply: %s", string(b))
	}
	var out struct {
		Amount struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"amount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Amount.Amount, nil
}

// IBCTotalEscrow returns the total amount of a denom escrowed in IBC transfer module.
func (c *Client) IBCTotalEscrow(denom string) (string, error) {
	u := c.base + "/ibc/apps/transfer/v1/denoms/" + url.PathEscape(denom) + "/total_escrow"
	resp, err := c.client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("lcd ibc escrow: %s", string(b))
	}
	var out struct {
		Amount struct {
			Amount string `json:"amount"`
		} `json:"amount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Amount.Amount, nil
}

// CommunityPool returns the community pool balance for the given denom as an integer string (truncated).
func (c *Client) CommunityPool(denom string) (string, error) {
	u := c.base + "/cosmos/distribution/v1beta1/community_pool"
	resp, err := c.client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("lcd community pool: %s", string(b))
	}
	var out struct {
		Pool []struct {
			Denom  string `json:"denom"`
			Amount string `json:"amount"`
		} `json:"pool"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	for _, p := range out.Pool {
		if p.Denom == denom {
			return decToIntString(p.Amount), nil
		}
	}
	return "0", nil
}

// BalanceByDenom returns balance for address/denom
func (c *Client) BalanceByDenom(address, denom string) (string, error) {
	u := c.base + "/cosmos/bank/v1beta1/balances/" + url.PathEscape(address) + "/by_denom?denom=" + url.QueryEscape(denom)
	resp, err := c.client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("lcd balance: %s", string(b))
	}
	var out struct {
		Balance struct {
			Amount string `json:"amount"`
		} `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Balance.Amount, nil
}

// IsModuleAccount makes a shallow check if account is a module account by querying account type string.
func (c *Client) IsModuleAccount(address string) (bool, error) {
	u := c.base + "/cosmos/auth/v1beta1/accounts/" + url.PathEscape(address)
	resp, err := c.client.Get(u)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("lcd account: %s", string(b))
	}
	var out struct {
		Account struct {
			Type string `json:"@type"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, err
	}
	return strings.Contains(out.Account.Type, "ModuleAccount"), nil
}

// ModuleAddressByName resolves a module account name to its address via LCD.
func (c *Client) ModuleAddressByName(name string) (string, error) {
	u := c.base + "/cosmos/auth/v1beta1/module_accounts/" + url.PathEscape(name)
	resp, err := c.client.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("lcd module account by name: %s", string(b))
	}
	var out struct {
		Account struct {
			BaseAccount struct {
				Address string `json:"address"`
			} `json:"base_account"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Account.BaseAccount.Address, nil
}

// AuthAccount fetches the raw account JSON and its type string for a given address.
func (c *Client) AuthAccount(address string) (json.RawMessage, string, error) {
	u := c.base + "/cosmos/auth/v1beta1/accounts/" + url.PathEscape(address)
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("lcd account: %s", string(b))
	}
	var outer struct {
		Account json.RawMessage `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&outer); err != nil {
		return nil, "", err
	}
	var t struct {
		Type string `json:"@type"`
	}
	_ = json.Unmarshal(outer.Account, &t)
	return outer.Account, t.Type, nil
}

// ClaimRecord represents a claimed account entry from the claim module endpoint.
type ClaimRecord struct {
	Address string
	Time    *time.Time
	Amount  string // amount for requested denom (e.g., ulume)
}

// ClaimListClaimed fetches claimed accounts for a tier (1..4). Best-effort parsing.
// It extracts the amount for the provided denom when available.
func (c *Client) ClaimListClaimed(tier int, denom string) ([]ClaimRecord, error) {
	u := fmt.Sprintf("%s/LumeraProtocol/lumera/claim/list_claimed/%d", c.base, tier)
	resp, err := c.client.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("lcd claim list_claimed: %s", string(b))
	}
	// Try multiple shapes (backward-compatible):
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	// New shape: top-level "claims" with fields including destAddress, claimTime, and balance array
	if v, ok := raw["claims"]; ok {
		var claims []struct {
			DestAddress string `json:"destAddress"`
			ClaimTime   string `json:"claimTime"`
			Balance     []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"balance"`
		}
		if err := json.Unmarshal(v, &claims); err == nil {
			recs := make([]ClaimRecord, 0, len(claims))
			for _, it := range claims {
				if it.DestAddress == "" {
					continue
				}
				var tptr *time.Time
				if it.ClaimTime != "" {
					var sec int64
					if _, err := fmt.Sscan(it.ClaimTime, &sec); err == nil && sec > 0 {
						t := time.Unix(sec, 0).UTC()
						tptr = &t
					}
				}
				amt := ""
				for _, b := range it.Balance {
					if b.Denom == denom {
						amt = b.Amount
						break
					}
				}
				recs = append(recs, ClaimRecord{Address: it.DestAddress, Time: tptr, Amount: amt})
			}
			return recs, nil
		}
	}
	// Fallbacks: arrays under other keys, with RFC3339 or numeric time.
	var arr []map[string]any
	if v, ok := raw["records"]; ok {
		_ = json.Unmarshal(v, &arr)
	} else if v, ok := raw["claimed"]; ok {
		_ = json.Unmarshal(v, &arr)
	} else if v, ok := raw["list"]; ok {
		_ = json.Unmarshal(v, &arr)
	}
	recs := make([]ClaimRecord, 0, len(arr))
	for _, item := range arr {
		addr := ""
		if a, ok := item["address"].(string); ok {
			addr = a
		} else if a, ok := item["addr"].(string); ok {
			addr = a
		} else if a, ok := item["destAddress"].(string); ok { // another possible field name
			addr = a
		}
		var tptr *time.Time
		if ts, ok := item["claim_time"].(string); ok {
			if tt, err := time.Parse(time.RFC3339, ts); err == nil {
				tptr = &tt
			} else {
				// try seconds since epoch as string
				var sec int64
				if _, err := fmt.Sscan(ts, &sec); err == nil && sec > 0 {
					t := time.Unix(sec, 0).UTC()
					tptr = &t
				}
			}
		} else if ts, ok := item["time"].(string); ok {
			if tt, err := time.Parse(time.RFC3339, ts); err == nil {
				tptr = &tt
			} else {
				var sec int64
				if _, err := fmt.Sscan(ts, &sec); err == nil && sec > 0 {
					t := time.Unix(sec, 0).UTC()
					tptr = &t
				}
			}
		} else if f, ok := item["time"].(float64); ok {
			sec := int64(f)
			t := time.Unix(sec, 0).UTC()
			tptr = &t
		}
		// try to parse amount from balance array
		amt := ""
		if braw, ok := item["balance"]; ok {
			if barr, ok2 := braw.([]interface{}); ok2 {
				for _, bi := range barr {
					if m, ok3 := bi.(map[string]interface{}); ok3 {
						d, _ := m["denom"].(string)
						a, _ := m["amount"].(string)
						if d == denom && a != "" {
							amt = a
							break
						}
					}
				}
			}
		}
		if addr != "" {
			recs = append(recs, ClaimRecord{Address: addr, Time: tptr, Amount: amt})
		}
	}
	return recs, nil
}

func parseInt(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(s, &n)
	return n, err
}
