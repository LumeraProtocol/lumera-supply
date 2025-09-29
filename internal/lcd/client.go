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

func parseInt(s string) (int64, error) {
	var n int64
	_, err := fmt.Sscan(s, &n)
	return n, err
}
