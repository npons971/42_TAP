.DEFAULT_GOAL := help
.PHONY: help install install-webkit clean build build-server run-server run-all test-server build-cli run-cli test test-cli-pty gui run-gui dev-gui build-gui check-gui test-gui build-frontend test-client test-integration test-all frontend-deps fetch-webkit

PROJECT_ROOT := $(CURDIR)
GO := $(PROJECT_ROOT)/go
NODE := $(PROJECT_ROOT)/node
NPM := $(PROJECT_ROOT)/npm
WAILS := $(PROJECT_ROOT)/wails

SERVER_ADDR ?= :4242
WORLD_FILE ?= data/world.json
CLI_ADDR ?= 127.0.0.1:4242
GUI_HOST ?= 127.0.0.1
GUI_PORT ?= 4242
GUI_SERVER ?= auto
GUI_TAGS := webkit2_41
GUI_DIR := Project/client-gui
export PATH := $(PROJECT_ROOT):$(PATH)
export GOPATH := $(CURDIR)/.go-work
export GOCACHE := $(CURDIR)/.go-cache
export npm_config_cache := $(PROJECT_ROOT)/.npm-cache

help:
	@printf '%s\n' '42 TAP — commandes principales' \
	  '  make run-all      Lance le serveur ici et les deux clients dans deux terminaux' \
	  '  make build        Compile le serveur, le CLI et le GUI' \
	  '  make gui          Compile et ouvre le GUI ; démarre un serveur local si nécessaire' \
	  '  make dev-gui      Même lancement avec rechargement du frontend' \
	  '  make build-gui    Compile le GUI sans ouvrir de fenêtre' \
	  '  make check-gui    Vérifie les outils locaux et GTK/WebKit' \
	  '  make run-server   Lance uniquement le serveur TCP' \
	  '  make run-cli      Lance le client terminal' \
	  '  make test-all     Tests serveur, clients et interface' \
	  '  make install      Installe les outils dans le dépôt' \
	  '' 'Serveur distant : make gui GUI_HOST=adresse GUI_PORT=4242 GUI_SERVER=off'

build: build-server build-cli build-gui

run-all:
	+bash scripts/run-all.sh "$(MAKE)" "$(SERVER_ADDR)" "$(WORLD_FILE)" "$(CLI_ADDR)" "$(GUI_HOST)" "$(GUI_PORT)"

gui: build-server build-gui
	$(NODE) scripts/run-gui.mjs --host "$(GUI_HOST)" --port "$(GUI_PORT)" --server "$(GUI_SERVER)" --world "$(WORLD_FILE)"

run-gui: gui

dev-gui: build-server install frontend-deps
	$(NODE) scripts/run-gui.mjs --dev --host "$(GUI_HOST)" --port "$(GUI_PORT)" --server "$(GUI_SERVER)" --world "$(WORLD_FILE)"

build-gui: install frontend-deps
	$(WAILS) build -tags "$(GUI_TAGS)"

check-gui:
	bash scripts/check_42_env.sh

build-server: go
	mkdir -p .build
	./go build -o .build/tap-server ./cmd/server

run-server: go
	./go run ./cmd/server -addr "$(SERVER_ADDR)" -world "$(WORLD_FILE)"

test-server: go
	@if test -x ./cc; then \
		CGO_ENABLED=1 CC="$(PROJECT_ROOT)/cc" ./go test -race ./internal/server; \
	else \
		./go test ./internal/server; \
	fi

build-cli: go
	mkdir -p .build
	./go build -o .build/tap-cli ./cmd/client-cli

run-cli: go
	./go run ./cmd/client-cli -addr "$(CLI_ADDR)"

test: go
	@if test -x ./cc; then \
		CGO_ENABLED=1 CC="$(PROJECT_ROOT)/cc" ./go test -race ./...; \
	else \
		./go test ./...; \
	fi

frontend-deps: $(GUI_DIR)/frontend/node_modules/.tap-installed

$(GUI_DIR)/frontend/node_modules/.tap-installed: $(GUI_DIR)/frontend/package.json $(GUI_DIR)/frontend/package-lock.json npm
	$(NPM) --prefix $(GUI_DIR)/frontend ci --no-audit --no-fund
	touch $@

build-frontend: frontend-deps
	./npm --prefix Project/client-gui/frontend run build

test-client: go
	@if test -x ./cc; then \
		cd $(GUI_DIR) && CGO_ENABLED=1 CC="$(PROJECT_ROOT)/cc" $(GO) test -race -v app.go app_test.go; \
	else \
		cd $(GUI_DIR) && $(GO) test -v app.go app_test.go; \
	fi

test-integration: test-client

test-gui: test-client build-frontend
	./node scripts/test-gui-protocol.mjs

test-cli-pty: build-cli
	python3 scripts/test-cli-pty.py

test-all: test-server test-client test-gui test-cli-pty

install: go npm gcc wails install-webkit

install-webkit: .webkit-sdk/.installed
	bash scripts/install-webkit.sh --check

fetch-webkit: node
	@. /etc/os-release; $(NODE) scripts/fetch-webkit.mjs "$$ID" "$${VERSION_CODENAME:-$$VERSION_ID}" $(WEBKIT_FETCH_ARGS)

# Wails binding generation removes CC; Go must still find a local gcc.
gcc: cc
	ln -sf cc gcc

install_files/zig-x86_64-linux-0.15.2.tar.xz:
	mkdir -p install_files
	curl -fL --retry 3 -o $@.tmp https://ziglang.org/download/0.15.2/zig-x86_64-linux-0.15.2.tar.xz && \
	  printf '%s  %s\n' 02aa270f183da276e5b5920b1dac44a63f1a49e55050ebde3aecc9eb82f93239 $@.tmp | sha256sum -c - && \
	  mv $@.tmp $@

.cc-sdk/zig: | install_files/zig-x86_64-linux-0.15.2.tar.xz
	mkdir -p .cc-sdk
	tar -xJf install_files/zig-x86_64-linux-0.15.2.tar.xz -C .cc-sdk --strip-components=1

cc: .cc-sdk/zig scripts/cc-local.sh
	printf '%s\n' '#!/bin/sh' 'repo="$$(CDPATH= cd -- "$$(dirname -- "$$0")" && pwd)"' \
	  'exec bash "$$repo/scripts/cc-local.sh" "$$@"' > cc
	chmod +x cc

install_files/go1.27.1.linux-amd64.tar.gz:
	mkdir -p install_files
	curl -fL --retry 3 -o $@.tmp https://go.dev/dl/go1.27.1.linux-amd64.tar.gz && \
	  printf '%s  %s\n' 63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445 $@.tmp | sha256sum -c - && \
	  mv $@.tmp $@

go: | install_files/go1.27.1.linux-amd64.tar.gz
	mkdir -p .go-sdk
	tar -xzf install_files/go1.27.1.linux-amd64.tar.gz -C .go-sdk
	ln -s .go-sdk/go/bin/go go

install_files/node-v22.22.0-linux-x64.tar.xz:
	mkdir -p install_files
	curl -fL --retry 3 -o $@.tmp https://nodejs.org/dist/v22.22.0/node-v22.22.0-linux-x64.tar.xz && \
	  printf '%s  %s\n' 9aa8e9d2298ab68c600bd6fb86a6c13bce11a4eca1ba9b39d79fa021755d7c37 $@.tmp | sha256sum -c - && \
	  mv $@.tmp $@

node: | install_files/node-v22.22.0-linux-x64.tar.xz
	mkdir -p .node-sdk
	tar -xJf install_files/node-v22.22.0-linux-x64.tar.xz -C .node-sdk --strip-components=1
	ln -s .node-sdk/bin/node node

npm: node Makefile
	printf '%s\n' '#!/bin/sh' 'repo="$$(CDPATH= cd -- "$$(dirname -- "$$0")" && pwd)"' \
	  'export PATH="$$repo:$$PATH"' \
	  'export npm_config_cache="$$repo/.npm-cache"' \
	  'exec "$$repo/.node-sdk/bin/node" "$$repo/.node-sdk/lib/node_modules/npm/bin/npm-cli.js" "$$@"' > npm
	chmod +x npm

wails: .go-work/bin/wails npm scripts/wails-local.sh Makefile
	printf '%s\n' '#!/bin/sh' 'repo="$$(CDPATH= cd -- "$$(dirname -- "$$0")" && pwd)"' \
	  'exec bash "$$repo/scripts/wails-local.sh" "$$@"' > wails
	chmod +x wails

.go-work/bin/wails: go
	GOPATH="$(CURDIR)/.go-work" GOCACHE="$(CURDIR)/.go-cache" ./go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

.webkit-sdk/.installed: scripts/install-webkit.sh scripts/fetch-webkit.mjs scripts/fetch-webkit-fedora.mjs node
	bash scripts/install-webkit.sh
	touch $@

clean:
	@if test -d .go-work/pkg/mod; then find .go-work/pkg/mod -type d -exec chmod u+w {} +; fi
	rm -rf .go-sdk .node-sdk .cc-sdk .cc-cache .go-work .go-cache .npm-cache .webkit-sdk go node npm cc gcc wails
	rm -rf install_files
