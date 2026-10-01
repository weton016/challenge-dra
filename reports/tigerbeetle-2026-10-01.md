# TigerBeetle ledger: QA, load, faults and audit — 2026-10-01

Branch `tigerbeetle`. Code: `f623b56` (migration) and `6d72255` (5 ms window, 2 GiB grid cache, no fingerprint on internal transfers, readiness by lookup, history count in the audit).

The 10-minute load runs used the `6d72255` code before two changes that are off the authorization path: readiness switched from `Nop` to a treasury lookup, and the audit gained `history_rows`. The faults, audits and the final `make qa` ran on `6d72255` itself.

## Environment

- macOS (darwin 25.5, Apple Silicon); Docker Desktop VM with 8 CPUs and 7.6 GiB.
- Go 1.26.0; k6 v2.2.0 (host) and `grafana/k6:latest` (in-network run).
- TigerBeetle 0.17.9, one replica, `--cache-grid=2GiB`, volume `tbdata`.
- Two API containers and the simulator. `TB_BATCH_WINDOW=5ms`. HTTP timeout 1.5 s; k6 client timeout 3 s.
- Load: `scripts/load.js`, open arrival rate, 2,000 req/s, `MODE=mixed` (account 1 alternates as source and destination; the peer is `2 + n % 999998`). Seed: 1,000,000 accounts with 10¹² cents each.

## Result

The target is met only by the in-network run. The 10-minute, 2,000 req/s, mixed load passed all four k6 thresholds with k6 inside the Compose network: `reports/load-tb-10m-mixed-innet.json`, k6 exit 0.

| Metric | Value |
|---|---|
| Requests | 1,200,001 (1,999.96/s) |
| p50 / p95 / p99 / max | 5.3 / 27.5 / 74.4 / 993.6 ms |
| HTTP failures | 0 |
| Checks `APPROVED` | 100% |
| Dropped iterations | 0 |

That run started on a fresh ledger: 1M seeded accounts plus 120,000 smoke transfers. Behaviour on a ledger with several million more transfers was worse in this environment (see run 6 below) and was not measured for 10 minutes.

## All runs, in order

| # | Run (`reports/load-*.json`) | Setup | p99 ms | max ms | Failed | Dropped | Verdict |
|---|---|---|---|---|---|---|---|
| 1 | `tb-smoke-30s` | right after seeding 1M, no window, 512 MiB | 3,002 | 3,071 | 7,523 | 3,652 | fail: post-seed compaction |
| 2 | `tb-smoke-30s-2` | same, a minute later | 51 | 129 | 0 | 0 | pass (30 s) |
| 3 | `tb-10m-mixed` | 10 min, no window, 512 MiB | 1,595 | 2,702 | 7,245 (7,237 × 503) | 1,479 | **fail** |
| 4 | `tb-diag-3m` | 3 min, per-second CSV | 660 | 1,923 | 96 | 0 | diagnostic: 0.3–1.8 s stalls |
| 5 | `tb-window-smoke-30s` | old volume (~5M transfers), window + 2 GiB, cold cache after restart | 566 | 818 | 0 | 0 | fail p99 |
| 6 | `tb-window-smoke-60s` | same volume, 60 s | 2,823 | 3,027 | 1,990 | 1,614 | **fail**: a 10 s stall at the start |
| 7 | `tb-fresh-smoke-60s` | fresh volume, seed 1M, window + 2 GiB | 94 | 348 | 0 | 0 | pass (60 s) |
| 8 | `tb-10m-mixed-window` | 10 min from the macOS host | 148 | 977 | 4 (`EOF`) | 0 | **fail**: 4 transport errors |
| 9 | `tb-10m-mixed-innet` | 10 min, k6 in the Compose network | 74 | 994 | 0 | 0 | **pass** |

All 503 responses in runs 1, 3 and 4 are "outcome unknown, retry the same id". None is a decline.

### What changed between failure and pass

A Go probe sent 2,000 single transfers/s straight to TigerBeetle, with no HTTP (runs not in `reports/`, numbers below):

- Fresh cluster, 1,000 accounts: p99 5.4 ms, max 65 ms.
- Real 1M-account cluster: p99 240 ms, max 606 ms.
- Fresh cluster, 1M seeded accounts, spread peers, 512 MiB cache: p99 1.11 s, max 1.71 s.
- Same, 2 GiB cache: p99 76 ms, max 515 ms.
- 2 GiB cache, alternating runs without a window: p99 130 ms, max 690 ms. With a 5 ms window: p99 34–51 ms, max 153–235 ms.
- Random `user_data_128` against zero, with the window: p99 98/41 ms against 38/36 ms. Max 216/180 ms against 230/77 ms.

Conclusions recorded in `docs/decisions.md`:

- The cache size and the coalescing window are what made the run pass.
- Dropping the fingerprint from internal transfers is a smaller gain.

### The 4 `EOF` errors in run 8

They were `Post "http://localhost:808x/transfers": EOF` on reused connections, at 11:55:31, 11:56:34, 11:59:38 and 12:00:27. Three of them fell in seconds with a max latency under 200 ms.

Things that are ruled out:

- **Idle timeouts:** k6 reuses VUs first in, first out (measured: every VU fires every 2.0 s), so no connection reaches the server's 30 s idle timeout.
- **Server errors:** the API logged no panic or server error.
- **Plain HTTP through the port forwarder:** a control run of 1,200,001 POSTs at 2,000/s from the host (`reports/load-http-control-10m.js`) had 0 transport errors and p99 51 ms. Every request had an empty body, so all were answered 400 without the ledger, and k6 counts all of them as `http_req_failed`. The `transport_errors` counter (status 0) never incremented, and the log has no warning.

The cause was not isolated. It shows only when ledger latency keeps more connections busy through Docker Desktop's port forwarder; run 9 bypasses that forwarder and had none. The Postgres-era run `load-check-10m-2000.json` also had 1 such failure in 1.2M.

## Faults (`reports/faults.json`, `reports/fault-manifest.json`)

All four checks passed with 203 unique operations, and every id was replayed on both APIs as APPROVED:

- **Committed response lost:** the replay returns APPROVED and creates no second effect.
- **SIGKILL of `api` with 200 concurrent requests:** all resolved on `api2`. `kill_initial_unknown: 0`: every request had finished before the kill, so this stage did not catch a request in flight.
- **Paused ledger:** answered 503 at 1,506 ms (no decline). After unpausing, the same id is APPROVED.
- **Ledger killed (`stop -t 0`) and restarted:** the data persisted, and the same id is APPROVED afterwards.

The first fault run failed at the ledger restart. `/readyz` used `Nop`, which succeeded while the replica was still recovering, so the next create timed out with a 503. Readiness now reads the treasury through the state machine; the rerun passed.

## Audit (`reports/audit-tigerbeetle.json`, traffic stopped)

The audit ran after load and faults:

- **Accounts:** 1,000,000 clients, 0 invalid balances.
- **Conservation:** money outside the mint equals `MaxInt64`; pending debits equal pending credits.
- **History:** 2,520,827 snapshots for accounts 1–3 replayed, 0 invalid prefixes, newest equals the account. This matches 120,000 + 1,200,001 + 1,200,001 load transfers on account 1, the seed transfers, and the fault and manual transfers.
- **External:** 0 unresolved reservations.

## QA (`make qa` on `6d72255`, with `make test-ledger`)

- **Formatting and vet:** gofmt clean; `go vet` clean.
- **Tests:** all pass with `-race`, `-p 1`, including the TigerBeetle integration, HTTP and Gherkin acceptance suites against the development cluster.
- **Coverage:** internal 95.3%, domain 100% (`reports/coverage.json`).
- **Mutation:** 12/12 selected financial mutants killed (`reports/mutations.json`).
- **Structure:** no violations — complexity ≤ 15, files ≤ 350 lines, packages ≤ 1,200, allowed imports (`reports/structure.json`, `reports/dependencies.json`).

## Limitations

- **Environment:** one machine and one replica on a Docker Desktop VM. That shows neither HA nor disk loss, nor production TigerBeetle hardware.
- **Ledger volume:** the pass is on a fresh ledger. A 60 s smoke on a ledger with about 5M transfers, right after a restart, stalled for 10 s.
- **Load generator placement:** the passing run's k6 shares the VM's CPUs with the system under test.
- **Audit scope:** the audit needs quiescence (no snapshot). It replays the history only of the chosen accounts and relies on TigerBeetle's double-entry structure for "two postings per approval".
