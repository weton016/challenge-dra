# QA procedure

## Preparation and isolation

`docker compose up -d --build` starts PostgreSQL, two APIs, and the simulator. `pix_test` is disposable: tests truncate only that database and refuse URLs outside the expected name. Do not point tests at real data. Load and audit use `pix`; the simulator uses `pix_external`. Integration tests do not run in parallel across packages (`-p 1`).

## Automatic gates

| Requirement | Command / evidence |
|---|---|
| Unit tests | `make unit`: cent limits, exact balance, insufficiency, overflow, and validation |
| Acceptance/Gherkin | `make acceptance`: feature file against real HTTP and PostgreSQL |
| Coverage | `make coverage`: minimum 85% of internal statements, 100% in the domain; HTML and JSON |
| Mutation | `make mutation`: nine compilable semantic changes in the monetary rules; all must be killed |
| Complexity | `make structure`: maximum 15 per internal production function, Go AST |
| Modules | Same command: maximum 350 lines per file and 1,200 per internal package |
| Dependencies | `go list` rejects cycles; AST prevents the domain from importing external libraries and persistence from importing HTTP |
| Metrics | JSON in reports: coverage, mutation, structure, latencies, throughput, and audit |
| QA | `make qa`: formatting, vet, race detector, tests, coverage, mutation, and structure |
| Final application | k6 + `python3 scripts/faults.py` + `make audit`, with ID reconciliation |

The GitHub Actions workflow runs `make qa` with real PostgreSQL and publishes the reports even on failure. Tests that depend on the database are explicitly skipped if run without TEST_DATABASE_URL; the validation command is therefore `make qa`, which sets that variable.

## Load and faults

1. `make seed` creates one million clients without replacing existing balances. The initial credit is an explicit fixture, not a public endpoint.
2. Warm up; run k6 with a new identifier per run, an open arrival rate, and sufficient funds. Do not use a constant-user executor to claim arrival capacity.
3. Repeat for inflow, outflow, and mixed traffic, recording environment, duration, p99, errors, and dropped iterations. Short results are diagnostic; they do not replace a sustained run.
4. The fault test takes down processes **of this Compose**: it interrupts one API with SIGKILL, pauses and restarts the database, and drops a response that was already committed. It does not delete volumes. Do not run it in a shared production environment.
5. Repeat every ID with an unknown result; compare the manifest with operations and postings. A 503 response never counts as DECLINED.
6. Audit history on a consistent snapshot: initial balances + postings, every prefix, sequences, conservation, and posting count, amount, and counterparties. Auditor limits: it checks the observed database; the manifest is required to detect a request that is entirely absent.

## Delivery criterion

A low p99 is not enough: a healthy run needs zero technical errors, zero dropped iterations, correct decisions, and a passing audit. Fault results are reported separately. Review documentation files and keep relevant negative results. Later changes on the financial path require a new validation.

## Limitations

Statement coverage does not measure every path. Nine selected mutants are not exhaustive mutation, especially of SQL. The tests are evidence for the invariants; the concurrency argument also depends on atomicity, locks, constraints, and the write protocol. Docker on one computer does not demonstrate HA, disk loss, or a real Pix integration.
