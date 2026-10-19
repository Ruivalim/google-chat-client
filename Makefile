# Google Chat Client: aplicativo desktop com múltiplas contas Google Workspace

SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
MAKEFLAGS += --warn-undefined-variables --no-builtin-rules --no-print-directory
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:

APP ?= google-chat-client

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
setup: ## Instala as dependências Go e da interface
	@echo "$(GREEN)▸ setup$(RESET)"
	npm ci
	go mod download

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
	gofmt -w main.go desktop.go internal/chat
	npm run fmt

.PHONY: lint
lint: generate ## Verifica tipos TypeScript e executa go vet
	@echo "$(GREEN)▸ lint$(RESET)"
	npm run lint
	go vet ./...

.PHONY: test
test: ## Executa testes locais de contas e integração HTTP simulada
	@echo "$(GREEN)▸ test$(RESET)"
	go test ./...

.PHONY: check
check: lint test build ## Executa as mesmas verificações usadas no CI
	@echo "$(GREEN)✓ check ok$(RESET)"

##@ Build

.PHONY: build
build: ## Empacota o aplicativo para a plataforma atual em build/
	@echo "$(GREEN)▸ build$(RESET)"
	npm run build

.PHONY: build-all
build-all: ## Empacota Linux x64, Windows x64 e macOS universal
	@echo "$(GREEN)▸ build-all$(RESET)"
	npm run build:all

.PHONY: clean
clean: ## Remove artefatos de compilação e desenvolvimento
	@echo "$(GREEN)▸ clean$(RESET)"
	rm -rf build dist .mygo coverage
