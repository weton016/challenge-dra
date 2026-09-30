GO ?= go
PYTHON ?= python3
export TEST_DATABASE_URL ?= postgres://pix:pix-local-only@localhost:55432/pix_test?sslmode=disable
export DATABASE_URL ?= postgres://pix:pix-local-only@localhost:55432/pix?sslmode=disable

.PHONY: up unit test acceptance coverage mutation structure qa audit seed load fmt
up:
	docker compose up -d --build
fmt:
	$(GO) fmt ./...
unit:
	$(GO) test -race ./internal/domain/... ./internal/application/...
test:
	$(GO) test -p 1 -race -count=1 ./...
acceptance:
	$(GO) test -count=1 -v ./tests/acceptance
coverage:
	mkdir -p reports
	$(GO) test -p 1 -race -count=1 -coverpkg=./internal/... -coverprofile=reports/coverage.out ./...
	$(GO) tool cover -html=reports/coverage.out -o reports/coverage.html
	$(PYTHON) scripts/coverage.py
mutation:
	GO=$(GO) $(PYTHON) scripts/mutations.py
structure:
	mkdir -p reports
	$(GO) list -deps -json ./internal/... > reports/dependencies.json
	$(GO) run ./cmd/quality
qa:
	mkdir -p reports
	@test -z "$$(gofmt -l internal cmd tests)" || (echo 'Run make fmt'; exit 1)
	$(GO) vet ./...
	$(MAKE) coverage
	$(MAKE) mutation
	$(MAKE) structure
audit:
	$(GO) run ./cmd/audit
seed:
	$(GO) run ./cmd/pix -seed 1000000
load:
	k6 run scripts/load.js
