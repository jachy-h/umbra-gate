# Makefile for umbragate (llm-gateway-lite).
#
# Build flow:
#   1. Build/compile the frontend (Vite)        -> internal/web/dist/
#   2. `go build` embeds that dir into the binary so it ships as a single file.
#
# Common targets:
#   make            -> web + go build (default)
#   make web        -> build the frontend only
#   make build      -> build the Go binary (assumes web assets exist)
#   make run        -> web + go run
#   make test       -> go test ./...
#   make clean      -> remove built artifacts

GO            ?= go
NPM           ?= npm
BINARY        ?= umbragate
PKG           ?= .
WEB_DIR       ?= web
WEB_DIST      ?= internal/web/dist
LDFLAGS       := -s -w
GOFLAGS       := -trimpath

.PHONY: all web build run test clean install check-web

all: web build

# Ensure the embedded dist directory exists with a placeholder so `go build`
# works even before the frontend has been built.
check-web:
	@mkdir -p $(WEB_DIST)
	@if [ ! -f $(WEB_DIST)/index.html ]; then echo "frontend not built; placeholder served" >&2; fi

# Build the frontend. The Vite config (web/vite.config.ts) outputs to
# ../internal/web/dist so the Go embed directive picks it up.
web: check-web
	cd $(WEB_DIR) && $(NPM) ci && $(NPM) run build

# Build the Go binary. Requires the web assets to exist (the placeholder
# index.html committed to internal/web/dist is enough for a minimal build).
build: check-web
	$(GO) build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BINARY) $(PKG)

run: web
	$(GO) run $(PKG)

test:
	$(GO) test ./...

clean:
	rm -f $(BINARY)
	rm -rf $(WEB_DIST)/assets