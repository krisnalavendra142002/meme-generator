SHELL := /bin/sh
OS := $(shell uname -s 2>/dev/null || echo unknown)
export PATH := $(HOME)/.bun/bin:$(PATH)

-include .env
export

POSTGRES_HOST ?= localhost
POSTGRES_PORT ?= 5432
POSTGRES_USER ?= meme
POSTGRES_PASSWORD ?= meme
POSTGRES_DB ?= meme_portal
REDIS_HOST ?= localhost
REDIS_PORT ?= 6379
S3_BUCKET ?= meme-portal
DATABASE_URL ?= postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@$(POSTGRES_HOST):$(POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable
MIGRATE_VERSION ?= v4.18.3
SQLC_VERSION ?= v1.30.0
AIR_VERSION ?= v1.67.4

.PHONY: setup dev ensure-deps install-go install-bun install-process-compose install-migrate ensure-migrate install-sqlc ensure-sqlc install-air ensure-air install-web-deps infra-up infra-down infra-down-v infra-logs dev-api dev-web migrate-new migrate-up migrate-down migrate-version migrate-force sqlc-generate build lint check test tidy

setup: ensure-deps
	@printf '\n[setup] semua dependency siap.\n'

dev: ensure-deps
	@PATH="$$PATH:$$(go env GOPATH)/bin" process-compose -f process-compose.yaml -p 8099 up $(if $(shell [ -t 0 ] && echo tty),,--tui=false)

ensure-deps:
	@command -v docker >/dev/null 2>&1 || { printf '\n[deps] docker wajib terpasang: https://docs.docker.com/get-docker/\n'; exit 1; }
	@docker compose version >/dev/null 2>&1 || { printf '\n[deps] plugin "docker compose" tidak ditemukan.\n'; exit 1; }
	@if ! docker info >/dev/null 2>&1; then \
		if [ "$(OS)" = "Darwin" ]; then \
			printf '\n[deps] Docker daemon belum berjalan, mencoba menyalakan...\n'; \
			if [ -d /Applications/OrbStack.app ]; then open -a OrbStack; \
			elif [ -d /Applications/Docker.app ]; then open -a Docker; \
			else printf '\n[deps] Nyalakan Docker Desktop/OrbStack manual lalu ulangi.\n'; exit 1; \
			fi; \
			i=0; while [ $$i -lt 60 ] && ! docker info >/dev/null 2>&1; do sleep 2; i=$$((i+1)); done; \
			docker info >/dev/null 2>&1 || { printf '\n[deps] Docker daemon belum siap setelah 120s.\n'; exit 1; }; \
			printf '[deps] Docker daemon siap.\n'; \
		else \
			printf '\n[deps] Docker daemon belum berjalan. Nyalakan manual (mis. sudo systemctl start docker) lalu ulangi.\n'; \
			exit 1; \
		fi; \
	fi
	@command -v go >/dev/null 2>&1 || $(MAKE) install-go
	@command -v bun >/dev/null 2>&1 || $(MAKE) install-bun
	@command -v process-compose >/dev/null 2>&1 || $(MAKE) install-process-compose
	@$(MAKE) --no-print-directory ensure-migrate
	@$(MAKE) --no-print-directory ensure-sqlc
	@$(MAKE) --no-print-directory ensure-air
	@[ -x web/node_modules/.bin/next ] || $(MAKE) install-web-deps

install-web-deps:
	@printf '\n[deps] dependency web belum ada, menjalankan bun install...\n'
	cd web && bun install

install-go:
	@printf '\n[deps] go tidak ditemukan.\n'
	@if [ "$(OS)" = "Darwin" ] && command -v brew >/dev/null 2>&1; then \
		printf '[deps] menginstall via brew...\n'; \
		brew install go; \
	else \
		printf '\n[deps] Auto-install hanya didukung di macOS (brew).\n'; \
		printf '[deps] Install go sendiri sesuai OS kamu:\n'; \
		printf '  - linux : paket distro (apt/dnf/pacman) atau https://go.dev/dl/\n'; \
		printf '  - macOS : brew install go\n'; \
		exit 1; \
	fi

install-bun:
	@printf '\n[deps] bun tidak ditemukan.\n'
	@if [ "$(OS)" = "Darwin" ] && command -v brew >/dev/null 2>&1; then \
		printf '[deps] menginstall via brew...\n'; \
		brew install oven-sh/bun/bun; \
	else \
		printf '\n[deps] Auto-install hanya didukung di macOS (brew).\n'; \
		printf '[deps] Install bun sendiri sesuai OS kamu:\n'; \
		printf '  - linux : curl -fsSL https://bun.sh/install | bash\n'; \
		printf '  - macOS : brew install oven-sh/bun/bun\n'; \
		exit 1; \
	fi

install-process-compose:
	@printf '\n[deps] process-compose tidak ditemukan.\n'
	@if [ "$(OS)" = "Darwin" ] && command -v brew >/dev/null 2>&1; then \
		printf '[deps] menginstall via brew...\n'; \
		brew install f1bonacc1/tap/process-compose; \
	else \
		printf '\n[deps] Install process-compose sendiri sesuai OS kamu:\n'; \
		printf '  - linux : https://github.com/F1bonacc1/process-compose (releases / go install)\n'; \
		printf '  - macOS : brew install f1bonacc1/tap/process-compose\n'; \
		exit 1; \
	fi

install-migrate:
	@printf '\n[deps] migrate CLI tidak ditemukan, menginstall $(MIGRATE_VERSION) via go install...\n'
	@command -v go >/dev/null 2>&1 || { printf '[deps] go belum terpasang. jalankan: make install-go\n'; exit 1; }
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)

ensure-migrate:
	@PATH="$$PATH:$$(go env GOPATH)/bin"; command -v migrate >/dev/null 2>&1 || $(MAKE) --no-print-directory install-migrate

install-sqlc:
	@printf '\n[deps] sqlc CLI tidak ditemukan, menginstall $(SQLC_VERSION) via go install...\n'
	@command -v go >/dev/null 2>&1 || { printf '[deps] go belum terpasang. jalankan: make install-go\n'; exit 1; }
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)

ensure-sqlc:
	@PATH="$$PATH:$$(go env GOPATH)/bin"; command -v sqlc >/dev/null 2>&1 || $(MAKE) --no-print-directory install-sqlc

install-air:
	@printf '\n[deps] air CLI tidak ditemukan, menginstall $(AIR_VERSION) via go install...\n'
	@command -v go >/dev/null 2>&1 || { printf '[deps] go belum terpasang. jalankan: make install-go\n'; exit 1; }
	go install github.com/air-verse/air@$(AIR_VERSION)

ensure-air:
	@PATH="$$PATH:$$(go env GOPATH)/bin"; command -v air >/dev/null 2>&1 || $(MAKE) --no-print-directory install-air

infra-up:
	docker compose up -d --wait postgres redis garage
	@printf '\n[infra] bucket $(S3_BUCKET) dipakai via GARAGE_DEFAULT_BUCKET.\n'

infra-down:
	docker compose down

infra-down-v:
	docker compose down -v

infra-logs:
	docker compose logs -f

dev-api: ensure-air
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" air

dev-web:
	cd web && bun run dev

migrate-new: ensure-migrate
	@[ -n "$(name)" ] || { printf 'cara pakai: make migrate-new name=nama_migration\n'; exit 1; }
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" migrate create -ext sql -dir migrations -seq $(name)

migrate-up: ensure-migrate
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down: ensure-migrate
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" migrate -path migrations -database "$(DATABASE_URL)" down $(if $(n),$(n),1)

migrate-version: ensure-migrate
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" migrate -path migrations -database "$(DATABASE_URL)" version

migrate-force: ensure-migrate
	@[ -n "$(v)" ] || { printf 'cara pakai: make migrate-force v=N\n'; exit 1; }
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" migrate -path migrations -database "$(DATABASE_URL)" force $(v)

sqlc-generate: ensure-sqlc
	cd api && PATH="$$PATH:$$(go env GOPATH)/bin" sqlc generate

build:
	cd api && go build -o bin/api ./cmd/server
	cd web && bun run build

lint:
	cd api && go vet ./...
	cd web && bun run lint

check:
	cd api && go build -o bin/api ./cmd/server
	cd web && bun run check

test:
	cd api && go test ./...

tidy:
	cd api && go mod tidy
