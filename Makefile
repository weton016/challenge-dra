GO ?= go
PYTHON ?= python3
export TEST_TB_ADDRESS ?= 127.0.0.1:3001
export TB_ADDRESS ?= 127.0.0.1:3000
# The TigerBeetle client archive is rejected by the new Apple linker.
ifeq ($(shell uname -s),Darwin)
export CGO_LDFLAGS ?= -Wl,-ld_classic
endif

.PHONY: up test-ledger unit test acceptance coverage mutation structure qa audit seed load fmt
up:
	docker compose up -d --build --wait
test-ledger:
	docker compose --profile test up -d --wait tigerbeetle-test
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
