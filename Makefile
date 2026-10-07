.PHONY: install clean build-server run-server test-server build-frontend test-client test-integration test-all

PROJECT_ROOT := $(CURDIR)
GO := $(PROJECT_ROOT)/go
NODE := $(PROJECT_ROOT)/node
NPM := $(PROJECT_ROOT)/npm
WAILS := $(PROJECT_ROOT)/wails

SERVER_ADDR ?= :4242

build-server:
	mkdir -p .build
	$(GO) build -o .build/tap-server ./cmd/server

run-server:
	$(GO) run ./cmd/server -addr "$(SERVER_ADDR)"

test-server:
	@if command -v gcc >/dev/null 2>&1; then \
		$(GO) test -race ./internal/server; \
	else \
		$(GO) test ./internal/server; \
	fi

build-frontend:
	$(NPM) --prefix Project/client-gui/frontend run build

test-client:
	cd Project/client-gui && GOPATH="$(PROJECT_ROOT)/.go-work" GOCACHE="$(PROJECT_ROOT)/.go-cache" $(GO) test -v .

test-integration: test-client

test-all: test-server test-client

install: go npm wails .webkit-sdk/.installed

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

wails: go npm Makefile
	GOPATH="$(CURDIR)/.go-work" GOCACHE="$(CURDIR)/.go-cache" ./go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
	printf '%s\n' '#!/bin/sh' 'repo="$$(CDPATH= cd -- "$$(dirname -- "$$0")" && pwd)"' \
	  'export PATH="$$repo:$$PATH"' \
	  'export GOPATH="$$repo/.go-work" GOCACHE="$$repo/.go-cache"' \
	  'export PKG_CONFIG_PATH="$$repo/.webkit-sdk/usr/lib64/pkgconfig:$$repo/.webkit-sdk/usr/lib/x86_64-linux-gnu/pkgconfig:$$repo/.webkit-sdk/usr/lib/pkgconfig:$$repo/.webkit-sdk/usr/share/pkgconfig:$${PKG_CONFIG_PATH:-}"' \
	  'export LD_LIBRARY_PATH="$$repo/.webkit-sdk/usr/lib64:$$repo/.webkit-sdk/usr/lib/x86_64-linux-gnu:$$repo/.webkit-sdk/usr/lib:$${LD_LIBRARY_PATH:-}"' \
	  'exec "$$repo/.go-work/bin/wails" "$$@"' > wails
	chmod +x wails

.webkit-sdk/.installed: scripts/install-webkit.sh
	bash scripts/install-webkit.sh
	touch $@

clean:
	@if test -d .go-work/pkg/mod; then find .go-work/pkg/mod -type d -exec chmod u+w {} +; fi
	rm -rf .go-sdk .node-sdk .go-work .go-cache .npm-cache .webkit-sdk go node npm wails
	rm -rf install_files
