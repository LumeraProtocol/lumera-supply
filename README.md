# Lumera Supply Service

A minimal HTTP service to expose **total**, **non-circulating**, **circulating**, and **max** supply figures for the Lumera blockchain, computed from on-chain data (Cosmos SDK LCD) and policy-driven lockups. Intended for use by exchanges, indexers, and market data providers.

## Features

- Endpoints using net/http only: `/total`, `/circulating`, `/max`, `/non_circulating`, `/healthz`
- Swagger/OpenAPI: `/docs` (Swagger UI), `/openapi.yaml`
- In-memory snapshot cache (TTL=60s) with background refresher and ETag
- Policy-driven allowlist (module accounts, disclosed lockups)
- IBC escrow included via `/ibc/apps/transfer/v1/denoms/{denom}/total_escrow`
- Vesting math engine for Delayed, Continuous, Periodic, Clawback, PermanentLocked (ready for integration)
- Rate limiting: 60 rpm (burst 120)

## Build & Run

### Build locally

```bash
go build -ldflags "-s -w \
    -X 'main.GitTag=$(git describe --tags --always --dirty)' \
    -X 'main.GitCommit=$(git rev-parse --short HEAD)'" \
  -o bin/lumera-supply ./cmd/lumera-supply
  
./bin/lumera-supply -addr=:8080 -lcd=https://lcd.lumera.io -policy=policy.json -denom=ulume
```

### Docker

```bash
docker build \
  --build-arg GIT_TAG="$(git describe --tags --always --dirty)" \
  --build-arg GIT_COMMIT="$(git rev-parse --short HEAD)" \
  -t lumera-supply:local .
  
docker run --rm -p 8080:8080 -e LUMERA_LCD_URL=https://lcd.lumera.io lumera-supply:local
```

Note: Do not use http://localhost:1317 for LUMERA_LCD_URL inside the container — "localhost" refers to the container itself. On Linux, if host.docker.internal is not available, use one of:

- Add host mapping: `--add-host=host.docker.internal:host-gateway`
- Or run with host networking (exposes all ports): `--network=host`

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

```json
{
  "denom": "ulume",
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "total": "1000000"
}
```

- `GET /circulating?denom=ulume`

```json
{
  "denom": "ulume",
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "circulating": "985000",
  "non_circulating": "15000"
}
```

- `GET /non_circulating?denom=ulume`

```json
{
  "denom": "ulume",
  "height": 123,
  "updated_at": "2025-09-28T22:30:00Z",
  "etag": "...",
  "non_circulating": { "sum": "...", "cohorts": [ ... ] }
}
```

- `GET /max?denom=ulume`

```json
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

```bash
curl -s 'https://api.lumera.org/circulating?denom=ulume' | jq
curl -s 'https://api.lumera.org/non_circulating?verbose=1' | jq
curl -s 'https://api.lumera.org/total' | jq
curl -s 'https://api.lumera.org/status' | jq
curl -s 'https://api.lumera.org/version' | jq
```

## Tests

```bash
go test ./...
```

- Vesting engine unit tests (golden-like checks for each type)
- Supply invariant test uses httptest LCD to verify `total = circulating + non_circulating`

## Reverse proxy (nginx)

To serve the API under https://api.lumera.io/supply/ behind nginx, use the provided config at deploy/nginx.conf. It proxies to the app on 127.0.0.1:8080 and sets the necessary X-Forwarded-* headers including X-Forwarded-Prefix so docs and OpenAPI work under the /supply prefix.

Quick setup (Debian/Ubuntu):

- sudo cp deploy/nginx.conf /etc/nginx/sites-available/lumera-supply.conf
- sudo ln -s /etc/nginx/sites-available/lumera-supply.conf /etc/nginx/sites-enabled/
- sudo nginx -t && sudo systemctl reload nginx

Start the app on port 8080:

- ./bin/lumera-supply -addr=:8080 -lcd=https://lcd.lumera.io -policy=policy.json -denom=ulume

Notes:

- server_name is api.lumera.io and the app is exposed under the /supply path.
- If you terminate TLS at nginx, keep proxy_set_header X-Forwarded-Proto $scheme; so the app generates https links in /openapi.yaml and /docs.

## Notes

- The current implementation treats user-created vesting accounts as circulating by default and only excludes cohorts provided by policy.
- Integration with chain vesting account types can be added in the cohort calculators using the provided vesting math engine.

## CLI Auditor Tool


Build the one-shot CLI that reproduces the API snapshot locally and prints a full JSON payload with totals and the non-circulating breakdown:

```bash
go build -ldflags "-s -w \
    -X 'main.GitTag=$(git describe --tags --always --dirty)' \
    -X 'main.GitCommit=$(git rev-parse --short HEAD)'" \
  -o bin/lumera-supply-cli ./cmd/lumera-supply-cli
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

```json
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

## Systemd service (native)

Run the service directly on the host (no Docker) and manage it with systemd.

1) Install the binary

```bash
sudo install -m 0755 bin/lumera-supply /usr/local/bin/lumera-supply
```

1) Install the unit and environment file

```bash
sudo cp deploy/lumera-supply-native.service /etc/systemd/system/
sudo cp deploy/lumera-supply-native.env /etc/default/lumera-supply-native
```

1) Configure

- Edit /etc/default/lumera-supply-native (HTTP addr, LCD URL, default denom, policy path).
- Ensure a policy file exists at /etc/lumera/policy.json (or set LUMERA_POLICY_PATH accordi>ngly).
- Optional: to run as a non-root user, create a user and uncomment User= in the unit.

1) Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now lumera-supply-native
```

1) Verify

```bash
systemctl status lumera-supply-native
journalctl -u lumera-supply-native -f
curl -s http://localhost:8080/healthz
```

Notes

- This unit depends on network-online.target.
- It works seamlessly with the provided nginx reverse proxy under /supply (deploy/nginx.conf).

## Systemd service (Docker)

Run the service as a managed Docker container with systemd.

1) Build or choose an image

- Local build (from this repo):

```bash
  docker build \
    --build-arg GIT_TAG="$(git describe --tags --always --dirty)" \
    --build-arg GIT_COMMIT="$(git rev-parse --short HEAD)" \
    -t lumera-supply:local .
```

- Or set IMAGE to a published tag in /etc/default/lumera-supply.

1) Install the unit

```bash
sudo cp deploy/lumera-supply.service /etc/systemd/system/
sudo cp deploy/lumera-supply.env /etc/default/lumera-supply
```

1) Configure

- Edit /etc/default/lumera-supply as needed (IMAGE, PORT, LCD URL, etc.).
- Ensure a policy file exists at /etc/lumera/policy.json (or adjust POLICY_HOST_PATH in /etc/default/lumera-supply). The container mounts it to /etc/lumera/policy.json inside the container.

1) Enable and start

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now lumera-supply
```

1) Verify

```bash
systemctl status lumera-supply
journalctl -u lumera-supply -f
curl -s http://localhost:8080/healthz
```

Notes

- The unit depends on docker.service and network-online.target.
- Logs are available via journalctl; you can customize EXTRA_DOCKER_ARGS in /etc/default/lumera-supply (e.g., networking, DNS, or log driver settings).
- If you serve the API behind nginx under /supply, use deploy/nginx.conf from this repo.
