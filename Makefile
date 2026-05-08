.PHONY: build test lint fmt vet clean

GO ?= go
BINARY := archigraph
LDFLAGS := -s -w \
  -X github.com/cajasmota/archigraph/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
  -X github.com/cajasmota/archigraph/internal/version.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
  -X github.com/cajasmota/archigraph/internal/version.Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

build:
	$(GO) build -ldflags='$(LDFLAGS)' -o $(BINARY) ./cmd/archigraph

test:
	$(GO) test -race -count=1 ./...

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

clean:
	rm -f $(BINARY)
