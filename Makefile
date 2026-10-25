include tools/versions.env

GO ?= go
GOWORK ?= off
export GOWORK

BIN_DIR := $(CURDIR)/.tmp/bin
export PATH := $(BIN_DIR):$(PATH)

.PHONY: help fmt-check lint test test-race test-db vuln secrets verify tools \
	check-secrets migrate-verify migrations-test coverage openapi-generate openapi-lint \
	mod-policy mod-policy-test

help:
	@echo "Targets:"
	@echo "  fmt-check         gofmt -l (fail if dirty)"
	@echo "  lint              go vet + staticcheck"
	@echo "  test              unit tests (no DB required)"
	@echo "  test-race         unit + race detector"
	@echo "  migrate-verify    migrations up/down/up against DATABASE_TEST_URL"
	@echo "  migrations-test   alias for migrate-verify"
	@echo "  test-db           migrate-verify + DB integration tests"
	@echo "  openapi-generate  regenerate docs/ from swag annotations"
	@echo "  openapi-lint      contract checks + fail on OpenAPI drift"
	@echo "  mod-policy        reject local filesystem replacements"
	@echo "  mod-policy-test   test allowed and rejected replacement fixtures"
	@echo "  vuln              govulncheck"
	@echo "  secrets           gitleaks (redacted)"
	@echo "  coverage          unit coverage profile (informational)"
	@echo "  verify            full local gate (same intent as CI)"

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
	@out="$$(gofmt -l cmd internal docs)"; \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

mod-policy:
	@chmod +x scripts/check-local-replaces.sh
	./scripts/check-local-replaces.sh

mod-policy-test:
	@chmod +x scripts/check-local-replaces.sh scripts/check-local-replaces-test.sh
	./scripts/check-local-replaces-test.sh

lint:
	$(GO) vet ./cmd/... ./internal/...
	# Run analyzers with the module toolchain so they match go.mod (1.26+/1.27).
	$(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION) ./cmd/... ./internal/...

test:
	DATABASE_TEST_URL= $(GO) test -count=1 ./internal/...

test-race:
	DATABASE_TEST_URL= $(GO) test -race -count=1 ./internal/...

migrate-verify migrations-test:
	@chmod +x scripts/migrate-verify.sh
	./scripts/migrate-verify.sh

test-db: migrate-verify
	$(GO) test -p 1 -race -count=1 ./internal/infra/secondary/repositories/postgres/refreshstore/ ./internal/infra/secondary/repositories/postgres/oidcstore/ ./internal/infra/secondary/repositories/postgres/auditstore/ ./internal/infra/secondary/repositories/postgres/settingsstore/ ./internal/infra/secondary/repositories/postgres/petstore/ ./internal/infra/secondary/repositories/postgres/tagstore/ ./internal/infra/secondary/repositories/postgres/notifystore/ ./internal/infra/secondary/repositories/postgres/dao/ ./internal/test/...

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

secrets check-secrets:
	@chmod +x scripts/check-secrets.sh scripts/secrets-smoke.sh
	./scripts/secrets-smoke.sh
	./scripts/check-secrets.sh

coverage:
	$(GO) test -count=1 -coverprofile=.tmp/coverage.out ./internal/...
	$(GO) tool cover -func=.tmp/coverage.out | tail -n 1

openapi-generate:
	@chmod +x scripts/openapi-generate.sh
	./scripts/openapi-generate.sh

openapi-lint:
	@chmod +x scripts/openapi-lint.sh
	./scripts/openapi-lint.sh

verify: fmt-check mod-policy mod-policy-test lint test test-race secrets vuln openapi-lint
	@if [ -z "$${DATABASE_TEST_URL:-}" ]; then \
		echo "DATABASE_TEST_URL is required for verify (use docker-compose.test.yml)"; \
		exit 1; \
	fi
	$(MAKE) test-db
	@echo "verify OK"
