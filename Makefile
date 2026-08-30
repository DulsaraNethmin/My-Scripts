# spinup — development Makefile
# Progress tracking lives in docs/TASKS.tsv, driven by scripts/progress.sh.

BINARY      := spinup
MODULE      := github.com/DulsaraNethmin/spinup
BIN_DIR     := bin
STACKS_DIR  := stacks
PROGRESS    := ./scripts/progress.sh
LINT_STACKS := ./scripts/lint-stacks.sh
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
GO          := go
HAS_GO_CODE := $(wildcard main.go)

.DEFAULT_GOAL := help
.PHONY: help status tasks next start done todo handoff build install run clean \
        test test-unit test-integration lint vet fmt tidy stacks-validate \
        stacks-lint stacks-list doctor snapshot merge check

## ---------------------------------------------------------------- help

help: ## Show this help
	@printf '\n\033[1mspinup\033[0m — make targets\n\n'
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
	  | sort \
	  | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@printf '\n  \033[2mProgress:\033[0m make status | make start ID=1.1 | make done ID=1.1 | make handoff\n\n'

## ---------------------------------------------------------- progress

status: ## Progress summary + what is next
	@$(PROGRESS) status

tasks: ## Full task ledger
	@$(PROGRESS) list

next: ## Print the next todo task
	@$(PROGRESS) next

start: ## Start a task: make start ID=1.1 (creates/checks out its branch)
	@test -n "$(ID)" || { echo "usage: make start ID=1.1"; exit 1; }
	@$(PROGRESS) start $(ID)

done: ## Mark a task done: make done ID=1.1
	@test -n "$(ID)" || { echo "usage: make done ID=1.1"; exit 1; }
	@$(PROGRESS) done $(ID)

todo: ## Reopen a task: make todo ID=1.1
	@test -n "$(ID)" || { echo "usage: make todo ID=1.1"; exit 1; }
	@$(PROGRESS) todo $(ID)

handoff: ## Print the handoff prompt for the next context (make handoff ID=1.1 for a specific task)
	@$(PROGRESS) handoff $(ID)

merge: ## Merge the current task branch into main with --no-ff (does not push)
	@branch=$$(git rev-parse --abbrev-ref HEAD); \
	if [ "$$branch" = "main" ]; then echo "already on main"; exit 1; fi; \
	git checkout main && git merge --no-ff "$$branch" -m "merge: $$branch" && \
	echo "merged $$branch into main — push is yours to run"

## -------------------------------------------------------------- go

build: ## Build the binary into bin/
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet (Phase 2, task 2.1) — nothing to build"
else
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) .
	@echo "built $(BIN_DIR)/$(BINARY) ($(VERSION))"
endif

install: ## go install the binary into GOPATH/bin
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — nothing to install"
else
	$(GO) install -trimpath -ldflags '$(LDFLAGS)' .
endif

run: build ## Build and run: make run ARGS="up postgres"
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — nothing to run"
else
	@$(BIN_DIR)/$(BINARY) $(ARGS)
endif

test: test-unit ## Run unit tests

test-unit: ## Run Go unit tests
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — no tests to run"
else
	$(GO) test ./... -race -count=1
endif

test-integration: ## Run tests that touch Docker (tag: integration)
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — no integration tests to run"
else
	$(GO) test ./... -race -count=1 -tags integration -timeout 10m
endif

lint: ## golangci-lint (skips if not installed)
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — nothing to lint"
else
	@command -v golangci-lint >/dev/null 2>&1 \
	  && golangci-lint run ./... \
	  || echo "golangci-lint not installed — brew install golangci-lint"
endif

vet: ## go vet
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — nothing to vet"
else
	$(GO) vet ./...
endif

fmt: ## gofmt -w
ifeq ($(HAS_GO_CODE),)
	@echo "no Go code yet — nothing to format"
else
	$(GO) fmt ./...
endif

tidy: ## go mod tidy
ifeq ($(HAS_GO_CODE),)
	@echo "no go.mod yet"
else
	$(GO) mod tidy
endif

## ---------------------------------------------------------- stacks

stacks-list: ## List stacks in the catalog
	@if [ -d $(STACKS_DIR) ]; then \
	  for d in $(STACKS_DIR)/*/; do [ -d "$$d" ] && basename "$$d"; done; \
	else echo "no $(STACKS_DIR)/ yet (Phase 1)"; fi

stacks-validate: ## docker compose config on every stack
	@if [ ! -d $(STACKS_DIR) ]; then echo "no $(STACKS_DIR)/ yet (Phase 1)"; exit 0; fi; \
	fail=0; \
	for d in $(STACKS_DIR)/*/; do \
	  name=$$(basename "$$d"); \
	  env=""; [ -f "$$d/.env.example" ] && env="--env-file $$d/.env.example"; \
	  if docker compose -f "$$d/compose.yaml" $$env config -q >/dev/null 2>&1; then \
	    printf '  \033[32mok\033[0m    %s\n' "$$name"; \
	  else \
	    printf '  \033[31mFAIL\033[0m  %s\n' "$$name"; \
	    docker compose -f "$$d/compose.yaml" $$env config -q 2>&1 | sed 's/^/        /'; \
	    fail=1; \
	  fi; \
	done; \
	exit $$fail

stacks-lint: ## Structural lint of the stack catalog
	@$(LINT_STACKS)

doctor: ## Check the local dev toolchain
	@printf '\n\033[1mdev environment\033[0m\n\n'
	@printf '  go       %s\n' "$$(go version 2>/dev/null || echo 'MISSING')"
	@printf '  docker   %s\n' "$$(docker --version 2>/dev/null || echo 'MISSING')"
	@printf '  compose  %s\n' "$$(docker compose version --short 2>/dev/null || echo 'MISSING')"
	@printf '  daemon   %s\n' "$$(docker info >/dev/null 2>&1 && echo 'running' || echo 'NOT running')"
	@printf '  lint     %s\n' "$$(command -v golangci-lint >/dev/null 2>&1 && golangci-lint --version | head -1 || echo 'not installed (optional)')"
	@printf '  release  %s\n' "$$(command -v goreleaser >/dev/null 2>&1 && goreleaser --version 2>/dev/null | grep -iE 'version:' | head -1 || echo 'not installed (optional)')"
	@printf '\n'

check: vet lint test stacks-lint stacks-validate ## Everything CI runs

snapshot: ## Local GoReleaser snapshot build
	@command -v goreleaser >/dev/null 2>&1 \
	  && goreleaser release --snapshot --clean \
	  || echo "goreleaser not installed — brew install goreleaser"

clean: ## Remove build artefacts
	@rm -rf $(BIN_DIR) dist
	@echo "cleaned"
