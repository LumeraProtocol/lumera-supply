# Lumera Circulating Supply Methodology & API

## Scope

This document defines how Lumera computes and publishes Total Supply, Circulating Supply, and Max Supply for the LUME asset, and specifies the HTTPS endpoints exchanges and data providers should query.

## Definitions

* **Total Supply (LUME):** On-chain total as reported by `x/bank` at block *H*.
* **Circulating Supply (LUME):** Total Supply minus balances that are not freely transferable at block *H*.
* **Max Supply:** Not applicable - no hard cap exists, `null`.
* **Staking treatment.** Staked but *unlocked* LUME **counts as circulating**. Only the *locked* portion of vesting/lockup accounts is excluded from circulating supply.
* **Default denom.** Unless otherwise specified via the `?denom` query parameter, all endpoints default to `ulume`.


## High-level formula

At block height **H**:
```
total(H)        = bank.supply(LUME)
non_circ(H)     = modules(H) + vesting_locked(H) + disclosed_lockups(H)
circulating(H)  = total(H) - non_circ(H)
```

### What counts as non-circulating

1. **Module & escrow accounts** (no private key; governance/logic-gated):
   - **1.1** Community Pool (distribution module) - `/cosmos/distribution/v1beta1/community_pool`
   - **1.2** Claim escrow (Claim module) - `/cosmos/bank/v1beta1/balances/MODULE_ADDRESS`
   - **1.3** IBC/ICS escrow accounts - `/ibc/apps/transfer/v1/denoms/ulume/total_escrow`
   - **1.4** Other protocol escrows that are different from `transfer` (DEX/auction escrows, if any)
2. **Protocol/foundation-originated vesting (locked portion only):**
   - **1.1** Genesis/foundation allocations with on-chain vesting - [policy.json](policy.json)
   - **1.2** Claimed “delayed” accounts (locked tranche) - `/LumeraProtocol/lumera/claim/list_claimed/1..4`
   - **1.3** Supernode bootstraps self-stake accounts (25k) created by protocol that vest over time - [policy.json](policy.json)
   - **1.4** Any governance-mandated timelocks - [policy.json](policy.json)
3. **Disclosed foundation/partner lockups** (if any) that are verifiably time-locked on the chain or governed by immutable timelock contracts - [policy.json](policy.json)

Notes:
* Only the **locked** portion is non-circulating at *H*.
* Staked coins remain circulating **if they are unlocked**; locking status, not staking status, drives circulation.

### What remains circulating

* Any balance in a normal account without transfer restrictions.
* Staked balances from unlocked accounts.
* **User-created vesting accounts** (e.g., `MsgCreateVestingAccount`) are treated as **circulating by default**. Rationale: anyone could self-lock to distort supply. **Exception:** if such accounts are explicitly listed in `policy/policy.json` under a vesting/lockup cohort, the **locked** portion is excluded.

## Community Pool

Always non-circulating while held by the distribution module. When governance spends from the pool to a standard account, that amount becomes circulating at the spend block.

Community pool amounts are published as DecCoins. We **truncate** (floor) to the integer base-denom amount of `ulume` at height H when computing non-circulating.

## Foundation Genesis Cohorts (provenance)

The following genesis cohorts exist:

| Cohort             |       Amount LUME | Start (mo) |         End (mo) | Custody             |
| ------------------ | ----------------: | ---------: | ---------------: | ------------------- |
| Seed Sale 1..5     |    5,000,000 each |          0 |    6/12/18/24/30 | Multisig            |
| Private Sale 1..6  |    6,250,000 each |          0 | 6/10/14/18/22/26 | Multisig            |
| Team 1..6          | 8,333,333.33 each |          0 | 7/13/19/25/31/37 | Multisig            |
| Advisors 1..5      |    1,250,000 each |          0 |   15/18/21/24/27 | Multisig            |
| Ecosystem Dev 1    |        20,000,000 |          0 |                0 | Single-sig (liquid) |
| Ecosystem Dev 2    |        11,250,000 |          0 |                0 | Single-sig (liquid) |
| Ecosystem Dev 3..7 |   11,250,000 each |          0 |       1/3/6/9/12 | Multisig            |
| Community Growth 1 |        12,500,000 |          0 |                0 | Single-sig (liquid) |
| Community Growth 2 |        12,500,000 |          1 |               12 | Multisig            |

**Policy application:**

* Cohorts with `End time = 0` are liquid at genesis → circulating (unless moved into a protocol escrow).
* All other cohorts are protocol/foundation vesting → subtract **only the locked portion** at height *H*, derived from the on-chain vesting state.
* Exact vesting schedules (Delayed/Periodic/Continuous/Clawback/custom) are authoritative **as encoded on-chain**, not inferred solely from the table.

## Exact vesting math (per account, denom = LUME)

We compute the **locked** portion at height/time H via the chain’s spendable logic:

Definitions
- **ov** = `original_vesting` (for ***LUME***) from `BaseVestingAccount`
- **now** = block time at **H**
- **V_rem(H)** = unvested amount at **now** per the account’s vesting schedule

Locked (non-circulating) portion:`locked(H) = V_rem(H)`

Per-type **V_rem(H)**
1) **DelayedVestingAccount**
```
    V_rem(H) = (now < end_time) ? ov : 0`
```
2) **ContinuousVestingAccount**
```
    Let T = end_time − start_time, t = clamp(now − start_time, 0, T)
    vested = ov * t / T
    V_rem(H) = ov − vested
```
3) **PeriodicVestingAccount** (and ClawbackVestingAccount)
```
    V_rem(H) = sum of all period amounts whose (cumulative) unlock time > now
```
4) **PermanentLockedAccount**
```
    V_rem(H) = ov    // never unlocks
```

**Notes:**
- **DelegatedVesting** and **DelegatedFree** do not change circulation status.
  Delegated (bonded) coins are circulating only if they are vested (i.e., not part of V_rem(H)).

**Global circulating** is:
 ```
 circulating(H) = total_supply(H) − [ Σ locked(H) over allowlisted protocol/foundation vesting accounts
                                      + module/escrow balances
                                      + disclosed immutable lockups ]
 ```
## API (stable, cacheable)

Base URL: `https://api.lumera.org/supply`

All responses include a single, consistent `height` (int64), `updated_at` (RFC3339), and `etag` (policy+inputs identifier). Values are base-denom integers.

Example:
```json
{
  "denom": "ulume",
  "decimals": 6,
  "amount": "1234567890123",
  "height": 1234567,
  "updated_at": "2025-09-29T22:11:33Z",
  "etag": "W/\"policy-v1.3-5c1a\""
}
```

### Endpoints

1. `GET /total?denom=ulume`
   Returns the latest snapshot including total supply, circulating, non_circulating sum, and max.
   ```
   {
     "denom": "ulume",
     "decimals": 6,
     "height": …,
     "updated_at": "…",
     "etag": "…",
     "total": "…",
     "circulating": "…",
     "non_circulating": "…",
     "max": null
   }
   ```

2. `GET /circulating?denom=ulume`
   ```
   {
     "denom": "ulume",
     "decimals":6,
     "height": …,
     "updated_at": "…",
     "etag": "",
     "circulating": "…",
     "non_circulating": "…"
   }
   ```

3. `GET /non_circulating?denom=ulume`
   Returns the **address lists** and per-cohort sums so auditors can reproduce the calculation at `height`.
   ```
   { 
      "denom":"ulume",
      "decimals":6,
      "height":…,
      "updated_at":"…",
      "non_circulating": {
        "sum": "179591804015777",
        "cohorts": [
        {
          "name": "ibc_escrow",
          "reason": "ICS20 transfer escrows",
          "amount": "200014020264"
        },
        {
          "name": "foundation_genesis",
          "reason": "protocol/foundation vesting locked portion",
          "items": [
            {
              "address": "lumera134tmfqteaytw30tpetkq65dnyx595wqqd0uf45",
              "amount": "5000000000000",
              "end_date": "2025-12-13T04:00:00Z"
            },
            ...
          ],
        },
        ...
        ]
      }          
   }
   ```

4. `GET /max?denom=ulume`
   ```
   { 
      "denom":"ulume",
      "decimals":6,
      "height":…,
      "updated_at":"…"
      "etag":"…"
      "amount":null,
   }
   ```

### Semantics & SLAs

* Numbers are computed at a specific block `height` and cached 60s.
* Strongly consistent within a response (single height across all fields).
* Rate limit: 60 req/min/IP (burst 120).
* Versioning via `ETag` and `Cache-Control: public, max-age=60`.

## Data publication & auditability

1. Machine-readable **allowlist** repository - URL
2. Historical JSON snapshots `{height,total,non_circulating_breakdown,circulating}` for audit - URL
