# Google Chat Client: aplicativo desktop com múltiplas contas Google Workspace

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables --no-builtin-rules --no-print-directory
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:

APP ?= google-chat-client
GO_DIRS := main.go desktop.go internal

BOLD  :=
CYAN  :=
GREEN :=
RESET :=
ifneq ($(shell [ -t 1 ] && echo tty),)
  BOLD  := $(shell tput bold 2>/dev/null)
  CYAN  := $(shell tput setaf 6 2>/dev/null)
  GREEN := $(shell tput setaf 2 2>/dev/null)
  RESET := $(shell tput sgr0 2>/dev/null)
endif

##@ Geral

.PHONY: help
help: ## Lista os alvos disponíveis
	@awk 'BEGIN { FS = ":.*##"; printf "\n$(BOLD)$(APP)$(RESET)\n\nUso: make $(CYAN)<alvo>$(RESET)\n" } \
	  /^##@/ { printf "\n$(BOLD)%s$(RESET)\n", substr($$0, 5) } \
	  /^[a-zA-Z0-9_.\/-]+:.*?##/ { printf "  $(CYAN)%-18s$(RESET) %s\n", $$1, $$2 } \
	  END { printf "\n" }' $(MAKEFILE_LIST)

.PHONY: setup
setup: ## Instala as dependências Go, da interface e o Chromium dos testes
	@echo "$(GREEN)▸ setup$(RESET)"
	npm ci
	go mod download
	npx playwright install chromium --only-shell

##@ Desenvolvimento

.PHONY: dev
dev: ## Abre o aplicativo desktop com recarga automática
	@echo "$(GREEN)▸ dev$(RESET)"
	npm run dev

.PHONY: preview
preview: generate ## Abre somente a prévia web da interface, sem contas Google
	@echo "$(GREEN)▸ preview$(RESET)"
	npm run dev:web

.PHONY: generate
generate: ## Gera a ponte TypeScript para os serviços Go
	@echo "$(GREEN)▸ generate$(RESET)"
	npm run generate

##@ Qualidade

.PHONY: fmt
fmt: ## Formata Go, TypeScript e CSS
	@echo "$(GREEN)▸ fmt$(RESET)"
	gofmt -w $(GO_DIRS)
	npm run fmt

.PHONY: lint
lint: generate ## Verifica formatação, tipos e lint de Go e TypeScript
	@echo "$(GREEN)▸ lint$(RESET)"
	@unformatted="$$(gofmt -l $(GO_DIRS))"; if [ -n "$$unformatted" ]; then echo "gofmt pendente:"; echo "$$unformatted"; exit 1; fi
	npm run lint
	go vet ./...

.PHONY: test
test: generate ## Testa o backend (Google simulado) e a interface (ponte simulada)
	@echo "$(GREEN)▸ test$(RESET)"
	go test -race ./...
	npm run test:ui

.PHONY: check
check: lint test build ## Executa as mesmas verificações usadas no CI
	@echo "$(GREEN)✓ check ok$(RESET)"

##@ Build

.PHONY: build
build: ## Empacota o aplicativo para a plataforma atual em build/
	@echo "$(GREEN)▸ build$(RESET)"
	npm run build

.PHONY: appimage
appimage: build ## Empacota o build Linux também como AppImage
	@echo "$(GREEN)▸ appimage$(RESET)"
	scripts/appimage.sh

.PHONY: build-all
build-all: ## Empacota Linux x64, Windows x64 e macOS universal
	@echo "$(GREEN)▸ build-all$(RESET)"
	npm run build:all

.PHONY: clean
clean: ## Remove artefatos de compilação e desenvolvimento
	@echo "$(GREEN)▸ clean$(RESET)"
	rm -rf build dist .mygo coverage
