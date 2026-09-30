# Pix transaction authorizer

Design v1.0 | 28 September 2026 | Document written before implementation

## 1. Scope, contract, and architecture

**Goal.** Authorize transfers between accounts with a balance that is never negative, a single effect per transaction, and balance and statement queries. The target is HTTP p99 < 300 ms with 1 million accounts and a peak of 2,000 requests/s touching the same account, which both receives and pays. Capacity and latency are hypotheses to validate under load; no benchmark result is claimed here.

**Assumptions.** BRL in integer cents, with no overdraft, foreign exchange, or fees. The debited source is a local account; the destination may be local or external. The authenticated caller may debit only permitted accounts. For an external destination, APPROVED means authorization and the local debit are persisted, not that the other institution has confirmed the credit. A real Pix integration is outside the prototype; a simulated, idempotent, queryable recipient represents that boundary.

| Component | Responsibility and reason |
| --- | --- |
| Modular monolith in Go | HTTP, validation, authorization, queries, and integration in one executable; reduces calls on the critical path and makes it easier to test and demonstrate in 48 h. Two instances exercise real concurrency. |
| Primary PostgreSQL | Single authority for decisions, balances, and history. ACID transactions and constraints keep debit, credit, statement, and idempotency together. A Docker volume preserves data after a process restart. |
| Outbox + dispatcher | Stores the intent to send externally in the financial commit and allows delivery to be retried after failures, without an external network call inside the transaction. |
| External simulator | Persists one effect per ID and answers state queries; allows timeout and resend to be demonstrated without claiming a real bank integration. |
| Docker Compose, k6, and auditor | Local reproduction, open-arrival traffic, and an independent check of the invariants after the load. |

**Internal path.** POST /transfers with a transaction_id generated and kept by the client, source, destination, and amount_minor. The API validates format and permission; runs one SQL transaction; responds APPROVED or DECLINED only after COMMIT. GET /transfers/{id} retrieves the decision. GET /accounts/{id}/balance reads the primary. GET /accounts/{id}/statement pages by account sequence; the first request fixes the maximum sequence for the following pages.

**SLO and unavailability.** p99 < 300 ms refers to conclusive decisions while the financial authority is healthy. If the database is unavailable for seconds, the service returns a technical error within a bounded time. If the connection was lost during COMMIT, the result is unknown; retry with the same ID or query it. A technical error is never a business decline. There is no authorization based on a cached balance.

<!-- pagebreak -->

## 2. Data, concurrency, and non-duplication

| Table | Main fields and indexes |
| --- | --- |
| accounts | id PK, kind CLIENT or EXTERNAL_CONTROL, currency, balance_minor CHECK >= 0, last_sequence. The database has 1 million client accounts and 32 outbound technical accounts, chosen by a stable hash of the source. |
| operations | transaction_id PK, principal_id, canonical payload, source_id, destination_id or destination_external, amount_minor, decision, reason, settlement_status, reverses_id UNIQUE when present, created_at. Records approvals and declines. |
| entries | operation_id FK, leg (DEBIT/CREDIT), account_id FK, delta_minor, account_sequence, balance_after_minor. UNIQUE(operation_id, leg) and UNIQUE(account_id, account_sequence). Immutable postings. |
| outbox | operation_id UNIQUE, external_request_id equal to transaction_id, payload, state, attempts, lease_until. A dispatcher resumes abandoned events with the same identity. |

**Invariants.** An approval has exactly two postings of the same amount and opposite signs; a decline has zero. Each balance equals the initial balance plus the sum of that account's postings. Every ordered prefix of the statement must have balance >= 0. The materialized balance is confirmed in the same COMMIT as the postings. The sum of client and technical-account balances stays equal to the initial sum of the local domain, taking reversals into account. A row CHECK protects against a negative balance; equality between postings requires the write protocol and is checked by the auditor. [1]

**Transaction per transfer: initial version.** Under READ COMMITTED, try to insert operations by the unique transaction_id. If the ID already exists, compare the payload and the principal: the same content returns the stored decision, different content returns a conflict. The UNIQUE constraint resolves two simultaneous requests with the same ID; the loser waits for the winner and rereads its result. For a new operation, lock source and destination with SELECT ... FOR NO KEY UPDATE in ascending ID order, one row at a time. Validate currency, existence, account type, a positive amount, and the absence of overflow. Debit only if the balance is sufficient; credit the destination; insert two postings and the decision. For insufficient balance, record DECLINED with no postings. COMMIT precedes the response. Credits and debits of the same account contend for its row, preserving a real order; independent accounts proceed in parallel. [2]

**Defense in depth.** On the debit write, condition UPDATE on balance_minor >= amount_minor and check that one row was changed. CHECK(balance_minor >= 0), unique indexes, and rollback prevent a partial state. Every financial movement, including a controlled seed and a reversal, goes through the same protocol. API roles do not receive direct UPDATE/DELETE permission on postings. The order of decisions is the effective order of locks and commits, with no FIFO promise based on network arrival.

**Idempotency.** The client reuses transaction_id after a timeout, including when it does not know whether a commit happened. A decline for insufficient balance is terminal for that ID; a new request requires a new ID. The key stays persisted, without depending on a cache TTL. If the application dies before the commit, there is no effect and no terminal result; if it dies after, both already exist. Querying an absent ID while an attempt is still in progress does not prove that it will not complete, so the client must repeat the same ID.

**External.** On authorization, the local source is debited and one of the non-spendable technical accounts receives the same amount, together with entries, operations APPROVED/EXTERNAL_PENDING, and the outbox. The technical shard keeps all external payments from contending for a single row. External confirmation changes settlement_status to SETTLED; a definitive rejection creates a single reversal operation linked to the original. SETTLED and REVERSED contend conditionally for the same operations row; the transition to REVERSED, the postings, and the balances commit under the same account locks. An external timeout keeps PENDING until reconciliation by ID. The technical control represents authorized local outflows net of reversals; it is not the recipient's real balance.

<!-- pagebreak -->

## 3. Hot accounts, targets, and failures

**Why the hot account is hard.** With 2,000 transfers/s involving one account, every transaction that changes its balance competes for the same record, including credits. Extra API instances do not remove that contention. The initial version uses short transactions, PK indexes, a limited connection pool, and no external call during the transaction. The external technical account is spread across 32 rows, but that does not solve contention on the hot source. The real capacity of this approach has to be measured.

**Decision conditioned on a benchmark.** First run the per-transfer version and record p99, throughput, and lock wait under 2,000 req/s on the same account. If it fails or has no capacity margin, implement microbatch as a localized optimization in the financial module: group at most 64 requests for up to 2 ms, record the batch order, validate each debit in sequence against intermediate balances, and write each account's final balance once, with individual decisions and postings in the same commit. A database transaction lock prevents two batch executors from running at the same time. Switching mode requires draining the previous writers; the two modes do not run simultaneously. The batch has its own deadline; one client's timeout does not cancel the others. This alternative can raise hot-account throughput, but it serializes batches of independent accounts; it is adopted only if measurement justifies it. Compare batches of 1, 16, and 64 before closing the implementation. [2]

| Required failure | Behavior and verification |
| --- | --- |
| Instance dies before COMMIT | PostgreSQL rolls back; locks are released after the connection ends. The client repeats the ID. The test kills one of the two instances under load. |
| Instance dies after COMMIT, before the response | Operation, balance, and postings exist together. A resend returns the previous decision, with no new movement. |
| Database is slow for seconds | Connection, lock, and transaction timeouts bound the wait; technical response. If COMMIT was sent and the response was lost, the result is unknown and must be reconciled by the same ID. |
| Database unavailable for seconds | Do not approve and do not record a financial decline in memory. Return a technical error, preserve requests already confirmed, and accept retries when it returns. |
| Client times out and resends | The same transaction_id produces the original result, or a single execution if the first aborted. A different payload with the same ID produces a conflict. |
| External recipient times out or the dispatcher crashes | The outbox is resumed; the simulator recognizes the same ID. Query state before reversing; an isolated timeout never releases funds. |

**Limits of the guarantee.** Compose with one database and a volume demonstrates process recovery, not disk loss or lossless failover. Production would need a backup policy, a synchronous replica if zero RPO is required, and protection against two primaries. Preserve fsync and synchronous_commit. No architecture can give a conclusive financial decision during seconds of unavailability of the only authoritative database and still meet p99 < 300 ms; those failures will be measured and reported separately. [3]

<!-- pagebreak -->

## 4. Testing, trade-offs, and plan

**Load test.** Preload 1 million accounts and record hardware, versions, and commit. Warm up and run for 10 min at 2,000 req/s that touch the same account, in separate fan-in, fan-out, and mixed inflow/outflow scenarios. Keep sufficient funds in the throughput run and add 200 req/s between cold accounts. Exercise concurrent queries and external payments in another run. k6 uses an open arrival rate; record offered, started, completed, and dropped_iterations, plus end-to-end HTTP p50/p95/p99, lock wait, timeouts, declines, and technical errors. The healthy scenario passes only with p99 < 300 ms, zero technical errors, and no dropped iterations; the report also states sustained throughput and the margin before saturation. [4]

**Independent audit.** Generate a manifest of IDs and payloads; after the load, reconcile unknown responses by repeating the same IDs and drain the simulator outbox. Recalculate each balance and each statement prefix from the seed and the entries. Check balance >= 0 on every prefix, the sum of balances, two opposite postings per approval, zero per decline, at most one initial movement per ID, at most one reversal per original, and a known result for every ID sent. Run 100 distinct withdrawals of R$ 10 against R$ 100: exactly 10 APPROVED, 90 DECLINED, and a zero balance; repeat the 100 keys with no new effect. Inject kill -9, database slowness, and a lost response after COMMIT.

**Trade-offs.** One transaction per transfer is simple to audit and parallelizes independent accounts, but the hot account may exceed the SLO because of contention. Microbatch reduces writes and commits on the hot account, at the cost of aggregation wait and broader serialization; adoption depends on the test. A single PostgreSQL simplifies atomicity and the statement, but concentrates the availability dependency. External send uses an outbox and compensation; there is no atomic commit across institutions. The simulator can prove recipient idempotency, not the availability or semantics of a real provider. Technical shards avoid an artificial bottleneck, adding only the deterministic choice of the technical account.

**Pending validations.** Demonstrate p99 at 2,000 req/s on the delivery hardware; confirm that the extra load does not starve; choose the number of instances, the pool, and the number of technical shards by measurement; test the cost of a hot-account statement and the growth of operations/entries; measure recovery time and outbox backlog after failures. If the baseline does not reach p99, document the numbers, apply the batch optimization, and compare without hiding failures. The 48 h target requires closing this cycle early.

**Delivery order.** This design must be sent to both evaluators before code starts and within 6 h of receiving the challenge. Then: implement the core, measure the baseline, decide on batching, complete queries and the simulator, run load and audit, preserve commits, and record AI prompts/sessions in the repository. The README will carry commands and real numeric results. This version replaces v0.2: it starts with one transaction per transfer and keeps microbatches as a testable hypothesis. No benchmark was run for this document.

**References.** [1] https://www.postgresql.org/docs/current/ddl-constraints.html ; [2] https://www.postgresql.org/docs/current/explicit-locking.html ; [3] https://www.postgresql.org/docs/current/wal-async-commit.html ; [4] https://grafana.com/docs/k6/latest/using-k6/scenarios/executors/constant-arrival-rate/
