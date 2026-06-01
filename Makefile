# Claudio developer automation.
#
# The default target is intentionally read-only. Build/test targets mirror the
# commands documented in CLAUDE.md and GitHub Actions.

GO ?= go
GOLANGCI_LINT ?= golangci-lint

APP := claudio
APP_CMD := ./cmd/claudio
HOOK_LOGGER := hook-logger
HOOK_LOGGER_CMD := ./cmd/hook-logger
PKGS := ./...
COVERAGE_PACKAGES := ./internal/hooks ./internal/install
COVER_PROFILE := coverage.out
COVER_HTML := coverage.html
SMOKE_CONFIG := config-example.json
SMOKE_SOUNDPACK := soundpacks/startrek-bridge
SMOKE_PAYLOAD := {"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}

HOST_GOOS := $(shell $(GO) env GOOS)
HOST_GOARCH := $(shell $(GO) env GOARCH)

ifeq ($(OS),Windows_NT)
SHELL := powershell.exe
.SHELLFLAGS := -NoProfile -ExecutionPolicy Bypass -Command
EXE := .exe
INSTALL_DIR ?= $(USERPROFILE)\bin
else
EXE :=
INSTALL_DIR ?= $(HOME)/bin
endif

BIN := $(APP)$(EXE)
HOOK_LOGGER_BIN := $(HOOK_LOGGER)$(EXE)
DIST_BIN := dist/$(APP)-$(HOST_GOOS)-$(HOST_GOARCH)$(EXE)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show available targets.
	@echo ""
	@echo "Claudio Makefile"
	@echo ""
	@echo "Core:"
	@echo "  make build             Build ./cmd/claudio for the current platform"
	@echo "  make test              Run the full Go test suite"
	@echo "  make ci                Run the local CI gate: vet, tests, builds, coverage"
	@echo "  make clean             Remove generated build, release, and coverage outputs"
	@echo ""
	@echo "Quality:"
	@echo "  make fmt               Format Go source under cmd/ and internal/"
	@echo "  make fmt-check         Fail when Go source needs formatting"
	@echo "  make vet               Run go vet ./..."
	@echo "  make lint              Run golangci-lint"
	@echo "  make tidy              Run go mod tidy"
	@echo ""
	@echo "Testing:"
	@echo "  make test-v            Run verbose uncached tests"
	@echo "  make test-race         Run the race detector"
	@echo "  make test-pkg PKG=...  Run one package, default PKG=./internal/cli"
	@echo "  make coverage          Write coverage.out"
	@echo "  make coverage-html     Write coverage.html"
	@echo "  make coverage-ci       Run the coverage packages used by CI"
	@echo ""
	@echo "Binaries:"
	@echo "  make build-all         Build claudio and hook-logger"
	@echo "  make build-nocgo       Verify CGO-free package build"
	@echo "  make install           Build and copy claudio to INSTALL_DIR=$(INSTALL_DIR)"
	@echo "  make release-build     Build stripped current-platform artifact in dist/"
	@echo ""
	@echo "Smoke:"
	@echo "  make smoke             Build and run a silent PostToolUse smoke test"
	@echo "  make smoke-debug       Smoke test with CLAUDIO_LOG_LEVEL=debug"
	@echo "  make version           Build and print claudio --version"
	@echo "  make doctor            Print local Go and git environment"
	@echo ""

.PHONY: all
all: vet test build ## Vet, test, and build.

.PHONY: ci
ci: vet test-v build build-nocgo coverage-ci ## Local approximation of GitHub CI.

.PHONY: doctor
doctor: ## Print toolchain and platform details.
	@$(GO) version
	@$(GO) env GOOS GOARCH CGO_ENABLED GOEXE
	@git --version

.PHONY: deps
deps: ## Download module dependencies.
	$(GO) mod download

.PHONY: tidy
tidy: ## Normalize go.mod and go.sum.
	$(GO) mod tidy

.PHONY: fmt
fmt: ## Format Go source under cmd/ and internal/.
	gofmt -w cmd internal

.PHONY: fmt-check
fmt-check: ## Fail when Go source under cmd/ or internal/ needs formatting.
ifeq ($(OS),Windows_NT)
	@$$files = gofmt -l cmd internal; if ($$files) { $$files; exit 1 }
else
	@files="$$(gofmt -l cmd internal)"; if [ -n "$$files" ]; then echo "$$files"; exit 1; fi
endif

.PHONY: vet
vet: ## Run go vet over every package.
	$(GO) vet $(PKGS)

.PHONY: lint
lint: ## Run golangci-lint over every package.
	$(GOLANGCI_LINT) run $(PKGS)

.PHONY: test
test: ## Run the full Go test suite.
	$(GO) test $(PKGS)

.PHONY: test-v
test-v: ## Run verbose uncached tests, matching CI's test shape.
	$(GO) test $(PKGS) -v -count=1

.PHONY: test-race
test-race: ## Run all tests with the Go race detector.
	$(GO) test -race $(PKGS) -count=1

PKG ?= ./internal/cli
.PHONY: test-pkg
test-pkg: ## Run one package: make test-pkg PKG=./internal/config
	$(GO) test $(PKG) -v -count=1

.PHONY: coverage
coverage: ## Run all tests and write coverage.out.
	$(GO) test $(PKGS) -count=1 -coverprofile=$(COVER_PROFILE)

.PHONY: coverage-html
coverage-html: coverage ## Render coverage.html from coverage.out.
	$(GO) tool cover -html=$(COVER_PROFILE) -o $(COVER_HTML)

.PHONY: coverage-ci
coverage-ci: ## Run the two coverage packages named in GitHub Actions.
	$(GO) test $(COVERAGE_PACKAGES) -count=1 -cover

.PHONY: build
build: ## Build the claudio binary for the current platform.
	$(GO) build -o $(BIN) $(APP_CMD)

.PHONY: build-hook-logger
build-hook-logger: ## Build the hook logger helper binary.
	$(GO) build -o $(HOOK_LOGGER_BIN) $(HOOK_LOGGER_CMD)

.PHONY: build-all
build-all: build build-hook-logger ## Build all command binaries.

.PHONY: build-nocgo
build-nocgo: ## Verify the package graph builds with CGO disabled.
ifeq ($(OS),Windows_NT)
	@$$env:CGO_ENABLED = '0'; $(GO) build $(PKGS); Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
else
	CGO_ENABLED=0 $(GO) build $(PKGS)
endif

.PHONY: install
install: build ## Copy claudio to INSTALL_DIR.
ifeq ($(OS),Windows_NT)
	@New-Item -ItemType Directory -Force -Path '$(INSTALL_DIR)' | Out-Null
	Copy-Item -Force '$(BIN)' '$(INSTALL_DIR)'
else
	mkdir -p "$(INSTALL_DIR)"
	cp "$(BIN)" "$(INSTALL_DIR)/"
endif

.PHONY: smoke
smoke: build ## Run a silent hook payload through the freshly built binary.
ifeq ($(OS),Windows_NT)
	@'$(SMOKE_PAYLOAD)' | ./$(BIN) --config $(SMOKE_CONFIG) --soundpack $(SMOKE_SOUNDPACK) --silent
else
	@printf '%s\n' '$(SMOKE_PAYLOAD)' | ./$(BIN) --config $(SMOKE_CONFIG) --soundpack $(SMOKE_SOUNDPACK) --silent
endif

.PHONY: smoke-debug
smoke-debug: build ## Run the smoke test with debug logging enabled.
ifeq ($(OS),Windows_NT)
	@$$env:CLAUDIO_LOG_LEVEL = 'debug'; '$(SMOKE_PAYLOAD)' | ./$(BIN) --config $(SMOKE_CONFIG) --soundpack $(SMOKE_SOUNDPACK) --silent; Remove-Item Env:CLAUDIO_LOG_LEVEL -ErrorAction SilentlyContinue
else
	@printf '%s\n' '$(SMOKE_PAYLOAD)' | CLAUDIO_LOG_LEVEL=debug ./$(BIN) --config $(SMOKE_CONFIG) --soundpack $(SMOKE_SOUNDPACK) --silent
endif

.PHONY: version
version: build ## Print the built binary version.
	./$(BIN) --version

.PHONY: release-check
release-check: clean test build smoke version ## Local release readiness check without tagging.

.PHONY: release-build
release-build: test ## Build a stripped release artifact for the current platform.
ifeq ($(OS),Windows_NT)
	@New-Item -ItemType Directory -Force -Path dist | Out-Null
else
	mkdir -p dist
endif
	$(GO) build -ldflags "-s -w" -o $(DIST_BIN) $(APP_CMD)

.PHONY: clean
clean: ## Remove generated build, release, and coverage outputs.
ifeq ($(OS),Windows_NT)
	@Remove-Item -Force -ErrorAction SilentlyContinue claudio, claudio.exe, claudio.click.exe, hook-logger, hook-logger.exe, test_cli.exe, $(COVER_PROFILE), $(COVER_HTML)
	@Remove-Item -Recurse -Force -ErrorAction SilentlyContinue dist
else
	rm -f claudio claudio.exe claudio.click.exe hook-logger hook-logger.exe test_cli.exe $(COVER_PROFILE) $(COVER_HTML)
	rm -rf dist
endif
