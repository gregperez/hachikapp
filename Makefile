include tools/versions.env

GO ?= go
GOWORK ?= off
export GOWORK

BIN_DIR := $(CURDIR)/.tmp/bin
export PATH := $(BIN_DIR):$(PATH)

.PHONY: help fmt-check lint test test-race test-db vuln secrets verify tools \
	check-secrets migrate-verify coverage

help:
	@echo "Targets:"
	@echo "  fmt-check    gofmt -l (fail if dirty)"
	@echo "  lint         go vet + golangci-lint"
	@echo "  test         unit tests (no DB required)"
	@echo "  test-race    unit + race detector"
	@echo "  test-db      migrations up/down/up + DB integration tests"
	@echo "  vuln         govulncheck"
	@echo "  secrets      gitleaks (redacted)"
	@echo "  coverage     unit coverage profile (informational)"
	@echo "  verify       full local gate (same intent as CI)"

tools: $(BIN_DIR)/golangci-lint $(BIN_DIR)/govulncheck $(BIN_DIR)/staticcheck

$(BIN_DIR)/golangci-lint:
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(BIN_DIR)/govulncheck:
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)

$(BIN_DIR)/staticcheck:
	@mkdir -p $(BIN_DIR)
	GOBIN=$(BIN_DIR) $(GO) install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)

fmt-check:
	@out="$$(gofmt -l cmd internal)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

lint:
	$(GO) vet ./cmd/... ./internal/...
	# Run analyzers with the module toolchain so they match go.mod (1.26+/1.27).
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./cmd/... ./internal/...

test:
	$(GO) test -count=1 ./internal/...

test-race:
	$(GO) test -race -count=1 ./internal/...

migrate-verify:
	@chmod +x scripts/migrate-verify.sh
	./scripts/migrate-verify.sh

test-db: migrate-verify
	$(GO) test -race -count=1 ./internal/infra/secondary/repositories/postgres/refreshstore/ ./internal/test/...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

secrets check-secrets:
	@chmod +x scripts/check-secrets.sh
	./scripts/check-secrets.sh

coverage:
	$(GO) test -count=1 -coverprofile=.tmp/coverage.out ./internal/...
	$(GO) tool cover -func=.tmp/coverage.out | tail -n 1

verify: fmt-check lint test test-race secrets vuln
	@if [ -z "$${DATABASE_TEST_URL:-}" ]; then \
		echo "DATABASE_TEST_URL is required for verify (use docker-compose.test.yml)"; \
		exit 1; \
	fi
	$(MAKE) test-db
	@echo "verify OK"
