# QA procedure

## Preparation and isolation

`docker compose up -d --build --wait` starts TigerBeetle, two APIs, and the simulator. `make test-ledger` starts a separate disposable TigerBeetle (`--development`, port 3001, `test` profile) that is reformatted on every start. TigerBeetle cannot truncate, so each test uses its own random account range and random UUIDs. Tests use `TEST_TB_ADDRESS`; do not point it at real data. Load, faults and audit use the main cluster; the simulator keeps its receipts on ledger 2. Integration tests do not run in parallel across packages (`-p 1`).

## Automatic gates

| Requirement | Command / evidence |
|---|---|
| Unit tests | `make unit`: cent limits, exact balance, insufficiency, overflow, and validation |
| Acceptance/Gherkin | `make acceptance`: feature file against real HTTP and TigerBeetle |
| Coverage | `make coverage`: minimum 85% of internal statements, 100% in the domain; HTML and JSON |
| Mutation | `make mutation`: twelve compilable semantic changes in the monetary rules (available balance, statement delta, shard); all must be killed |
| Complexity | `make structure`: maximum 15 per internal production function, Go AST |
| Modules | Same command: maximum 350 lines per file and 1,200 per internal package |
| Dependencies | `go list` rejects cycles; AST prevents the domain from importing external libraries and infrastructure from importing HTTP |
| Metrics | JSON in reports: coverage, mutation, structure, latencies, throughput, and audit |
| QA | `make qa`: formatting, vet, race detector, tests, coverage, mutation, and structure |
| Final application | k6 + `python3 scripts/faults.py` + `make audit`, with ID reconciliation |

The GitHub Actions workflow runs `make test-ledger` and `make qa` against a real TigerBeetle and publishes the reports even on failure. Tests that depend on the ledger are explicitly skipped without TEST_TB_ADDRESS; the validation command is therefore `make qa`, which sets that variable.

## Load and faults

1. `make seed` creates one million clients without replacing existing balances. The initial credit is an explicit fixture, not a public endpoint.
2. Warm up; run k6 with a new identifier per run, an open arrival rate, and sufficient funds. Do not use a constant-user executor to claim arrival capacity.
3. Repeat for inflow, outflow, and mixed traffic, recording environment, duration, p99, errors, and dropped iterations. Short results are diagnostic; they do not replace a sustained run.
4. The fault test takes down processes **of this Compose**: it interrupts one API with SIGKILL, pauses and kills the TigerBeetle process, and drops a response that was already committed. It does not delete volumes. Do not run it in a shared production environment.
5. Repeat every ID with an unknown result; the manifest records each payload. A 503 response never counts as DECLINED.
6. Audit with traffic stopped (TigerBeetle has no read snapshot): every balance non-negative and within int64, money outside the mint equal to the fixed supply, pending debits equal to pending credits, every history prefix of the chosen accounts, and unresolved external reservations. Auditor limits: it checks the observed ledger; the manifest is required to detect a request that is entirely absent.

## Delivery criterion

A low p99 is not enough: a healthy run needs zero technical errors, zero dropped iterations, correct decisions, and a passing audit. Fault results are reported separately. Review documentation files and keep relevant negative results. Later changes on the financial path require a new validation.

## Limitations

Statement coverage does not measure every path. Twelve selected mutants are not exhaustive mutation, especially of the TigerBeetle adapter. The tests are evidence for the invariants; the concurrency argument also depends on TigerBeetle's atomic, serial commit and account flags. Docker on one computer does not demonstrate HA, disk loss, or a real Pix integration.
