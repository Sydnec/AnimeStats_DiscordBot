BINARY      := animestats
PKG         := ./cmd/animestats
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
GOFLAGS     := -trimpath

export CGO_ENABLED := 0

.PHONY: all build test lint fmt vet run clean dist

all: lint test build

build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

test:
	go test ./... -race -cover

lint: fmt vet

fmt:
	@test -z "$$(gofmt -l cmd internal)" || (echo "gofmt requis sur :"; gofmt -l cmd internal; exit 1)

vet:
	go vet ./...

run:
	go run $(PKG)

clean:
	rm -rf bin dist

# Binaires de release pour les deux architectures visées par le LXC.
dist: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-amd64 $(PKG)
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-arm64 $(PKG)
	cd dist && sha256sum $(BINARY)-linux-* > SHA256SUMS
