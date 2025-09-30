# Lumera Supply Service

A minimal HTTP service to expose **total**, **non-circulating**, **circulating**, and **max** supply figures for the Lumera blockchain, computed from on-chain data (Cosmos SDK LCD) and policy-driven lockups. Intended for use by exchanges, indexers, and market data providers.

## Features

- Endpoints using net/http only: `/total`, `/circulating`, `/max`, `/non_circulating`, `/healthz`
- In-memory snapshot cache (TTL=60s) with background refresher and ETag
- Policy-driven allowlist (module accounts, disclosed lockups)
- IBC escrow included via `/ibc/apps/transfer/v1/denoms/{denom}/total_escrow`
- Vesting math engine for Delayed, Continuous, Periodic, Clawback, PermanentLocked (ready for integration)
- Rate limiting: 60 rpm (burst 120)

## Build & Run

### Build locally
```
go build -o bin/lumera-supply ./cmd/lumera-supply
./bin/lumera-supply -addr=:8080 -lcd=https://lcd.lumera.io -policy=policy.json -denom=ulume
```

### Docker
```
docker build -t lumera-supply:local .
docker run --rm -p 8080:8080 -e LUMERA_LCD_URL=https://lcd.lumera.io lumera-supply:local
```

Note: Do not use http://localhost:1317 for LUMERA_LCD_URL inside the container — "localhost" refers to the container itself. On Linux, if host.docker.internal is not available, use one of:
- Add host mapping: --add-host=host.docker.internal:host-gateway
- Or run with host networking (exposes all ports): --network=host

Configuration
- LCD URL: `-lcd` flag or `LUMERA_LCD_URL`
- Policy path: `-policy` flag or `LUMERA_POLICY_PATH` (see `policy.example.json`)
- Default denom: `-denom` flag or `LUMERA_DEFAULT_DENOM` (default `ulume`)
- HTTP listen: `-addr` flag or `LUMERA_HTTP_ADDR`

## API

All endpoints accept `?denom=ulume` (default from config). Responses include headers:
- `ETag`
- `X-Block-Height`
- `X-Updated-At`

- `GET /total?denom=ulume`
```
{
  "denom": "ulume",
  "decimals": 6,
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "policy-etag": "...",
  "total": "1000000",
  "circulating": "...",
  "non_circulating": "...",
  "max": null
}
```

- `GET /circulating?denom=ulume`
```
{
  "denom": "ulume",
  "decimals": 6,
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "policy-etag": "...",
  "circulating": "985000",
  "non_circulating": "15000"
}
```

- `GET /non_circulating?denom=ulume`
```
{
  "denom": "ulume",
  "decimals": 6,
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "policy-etag": "...",
  "non_circulating": { "sum": "...", "cohorts": [ ... ] }
}
```

- `GET /max?denom=ulume`
```
{
  "denom": "ulume",
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "max": null
}
```

- `GET /healthz` → `{ "status": "ok", "time": "..." }`

## Quick examples

```
curl -s 'https://api.lumera.org/circulating?denom=ulume' | jq
curl -s 'https://api.lumera.org/non_circulating?verbose=1' | jq
curl -s 'https://api.lumera.org/total' | jq
curl -s 'https://api.lumera.org/status' | jq
curl -s 'https://api.lumera.org/version' | jq
```


## Tests
```
go test ./...
```
- Vesting engine unit tests (golden-like checks for each type)
- Supply invariant test uses httptest LCD to verify `total = circulating + non_circulating`

## Notes
- The current implementation treats user-created vesting accounts as circulating by default and only excludes cohorts provided by policy.
- Integration with chain vesting account types can be added in the cohort calculators using the provided vesting math engine.

## CLI Auditor Tool

Build the one-shot CLI that reproduces the API snapshot locally and prints a full JSON payload with totals and the non-circulating breakdown:

```bash
go build -o bin/lumera-supply-cli ./cmd/lumera-supply-cli
```

Run it (uses the same LCD and policy as the server):

```bash
./bin/lumera-supply-cli -lcd=https://lcd.lumera.io -policy=policy.json -denom=ulume
```

Environment variable equivalents:
- LUMERA_LCD_URL
- LUMERA_POLICY_PATH
- LUMERA_DEFAULT_DENOM

Output shape (pretty-printed):
```
{
  "denom": "ulume",
  "decimals": 6,
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "total": "...",
  "circulating": "...",
  "non_circulating": {
    "sum": "...",
    "cohorts": [ { "name": "ibc_escrow", "amount": "..." }, ... ]
  },
  "max": null
}
```
