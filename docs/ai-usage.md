# AI usage history

A short record of the main requests and decisions in this session; it is not a complete literal export. It contains no credentials for real services.

## Main candidate requests

1. Analyze the Pix authorizer challenge and choose architecture, language, and tools; hot-account support is mandatory.
2. Assess whether 2,000 requests/s on the same account with p99 under 300 ms is sustainable; explain microbatches and distributed coordination.
3. Compare a monolith and microservices.
4. Redo the design in at most four pages covering components, data model, concurrency, failures, trade-offs, and validations.
5. Implement the project with unit tests, Gherkin, coverage, mutation, cyclomatic complexity, module limits, dependencies, metrics, QA procedures, and final tests.
6. Continue and finish all of the work.

## Use of assistance

- Codex helped draft the design and choose Go/PostgreSQL with a modular monolith.
- The initial design was saved and committed before the code. Microbatches remained conditional on the baseline measurement.
- The assistant wrote code, tests, infrastructure, load/audit scripts, and documentation; it ran the commands in the local environment.
- The local skill `skills/pix-quality/SKILL.md`, discovered through AGENTS.md, records the gates requested by the candidate. The Ponytail skill guided avoiding unnecessary components; skill-creator guided the local skill; PDF guided rendering and inspecting the design.
- Review found and fixed a potential collision between a reversal identity and a client identity by reserving `refund:`.
- Results are taken from real runs. Compilation, infrastructure, or target failures are recorded and either fixed or stated; no benchmark is inferred from the architecture alone.

## Responsibility in the presentation

The candidate must review the code, understand READ COMMITTED and ordered locks, distinguish a financial decision from a technical error, explain the external settlement boundary, and reproduce the README commands. The assistant did not email the evaluators.
