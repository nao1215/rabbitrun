.PHONY: build run test e2e vet fmt lint vuln licenses tools clean help

APP         = rabbitrun
VERSION     = $(shell git describe --tags --abbrev=0 2>/dev/null || echo dev)
GO          = go
GO_LDFLAGS  = -ldflags '-X main.version=$(VERSION)'

# Tool versions; keep them equal to the versions the CI workflows pin.
GOLANGCI_LINT_VERSION = v2.14.0
GOVULNCHECK_VERSION   = v1.8.0
GO_LICENSES_VERSION   = v2.0.1

build: ## Build the rabbitrun binary
	$(GO) build $(GO_LDFLAGS) -o $(APP) .

run: build ## Build and start the game
	./$(APP)

test: ## Run unit tests with coverage (writes cover.out / cover.html)
	$(GO) test -race -cover -coverprofile=cover.out ./...
	$(GO) tool cover -html=cover.out -o cover.html

e2e: ## Run the atago end-to-end tests (requires atago; set RABBITRUN_E2E_DISPLAY=1 to include the specs that open a window)
	$(GO) run ./e2e/runner

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format Go source code
	$(GO) fmt ./...

lint: ## Run golangci-lint (same config as CI)
	golangci-lint run ./...

vuln: ## Scan for known vulnerabilities (requires govulncheck)
	govulncheck ./...

# GOROOT is set because go-licenses recognizes the standard library by path:
# after a toolchain switch it would report every standard package as unlicensed.
licenses: ## Check dependency licenses for each release OS and collect their texts (requires go-licenses)
	for goos in linux darwin windows; do \
		GOROOT="$$($(GO) env GOROOT)" GOOS=$$goos CGO_ENABLED=0 go-licenses check --disallowed_types=forbidden,restricted,unknown . || exit 1; \
	done
	./scripts/third_party_licenses.sh

tools: ## Install golangci-lint, govulncheck and go-licenses at the versions CI uses
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$(GO) install github.com/google/go-licenses/v2@$(GO_LICENSES_VERSION)

clean: ## Remove build and coverage output
	-rm -rf $(APP) $(APP).exe cover.out cover.html dist third_party_licenses

.DEFAULT_GOAL := help
help:
	@grep -E '^[0-9a-zA-Z_-]+[[:blank:]]*:.*?## .*$$' $(MAKEFILE_LIST) | sort \
	| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[1;32m%-15s\033[0m %s\n", $$1, $$2}'
