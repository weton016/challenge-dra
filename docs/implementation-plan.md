# Implementation plan and completion criteria

Design delivered in this conversation before the code: `design.md` and PDF. Emailing the evaluators remains the candidate's responsibility.

## Stages
1. Record the skill, criteria, and design in a commit.
2. Implement the Go monolith, schema/migrations, the financial transaction, balance/statement, and an outbox with an idempotent simulator.
3. Implement unit tests and Gherkin acceptance, with a real database and concurrency.
4. Automate the gates: internal coverage >= 85%, ledger 100%, financial mutants 100%, complexity <= 15, size, and dependencies.
5. Run QA with the race detector; fix failures without weakening invariants.
6. Bring up two APIs via Compose, simulate application death, unavailability, and resend, then run load and the independent auditor.
7. Measure 2,000 req/s on a hot account, document the results and any limit. Update the README, decisions, and AI history.

## Mapping of the ten rules
| Rule | Evidence |
|---|---|
| Unit tests | `go test`, validation/limit/decision tests |
| Acceptance/Gherkin | Godog with HTTP and PostgreSQL |
| Coverage | Go profiles and a per-package report |
| Mutation testing | selected semantic mutants in the financial core |
| Cyclomatic complexity | AST analysis of functions |
| Module size | line limits per file/package |
| Dependencies | real import graph and layer rules |
| Quality metrics | QA and benchmark report, with environment |
| QA procedures | Makefile + CI pipeline + fault script |
| Final tests | Compose, acceptance, faults, load, and audit |

Test configuration uses a separate disposable database. Destructive scenarios never run against an arbitrary production URL. There is no automatic repository publication or email.
