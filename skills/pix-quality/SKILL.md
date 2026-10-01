---
name: pix-quality
description: Implement and review this Pix authorizer with financial tests, Gherkin acceptance, coverage, mutation, and quality gates. Use when changing code in this repository.
---

# Pix authorizer quality

Use `docs/implementation-plan.md` as the checklist. Before finishing, run `make qa` and the final tests documented in the README. Record failures and limitations in `reports/`, without lowering limits to hide regressions.

- Unit tests must cover monetary limits, validation, and decisions, including the exact balance amount and overflow.
- `.feature` scenarios must run against real HTTP and TigerBeetle; isolated Gherkin documentation is not enough.
- Internal code coverage: >= 85%; financial domain: 100% of statements. Report the limits of what that metric measures.
- Mutation testing: kill 100% of the selected financial mutants; distinguish survivors, compilation errors, and infrastructure failures. The selected suite does not represent every possible mutant.
- Maximum cyclomatic complexity of 15 per function; production files <= 350 lines and internal packages <= 1,200 lines. Split by responsibility, without creating abstractions only to satisfy the numbers.
- Dependencies: packages under internal/domain use the standard library and import only other packages in that directory; the application depends only on the domain; infrastructure may use domain, application, and the TigerBeetle Go client, and does not import presentation; presentation uses application, domain, and infrastructure; no cycles. Validate with `go list`, not by design alone.
- Run gofmt, go vet, the race detector, tests, coverage, mutation, and the structural check. Store reports with date and commit.
- Use a real TigerBeetle cluster (`make test-ledger`) for concurrency, idempotency, failure, and audit tests. No negative account, two postings per approval, no effect on a decline, one effect per ID.
- Final test: bring up Compose, check readiness, run acceptance, faults, and load, reconcile operations, and audit balance and history. Report p99 and errors together; the 2,000 req/s target is met only if measured.
- Commit timeouts are indeterminate; never turn them into financial declines. An external reversal requires a definitive rejection and is atomic with the final state.
