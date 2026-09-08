BINARY  := animestats
PKG     := ./cmd/animestats
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath

.PHONY: all build test cover lint fmt vet run clean dist

all: lint test build

# CGO est désactivé uniquement pour la compilation : c'est ce qui produit un
# binaire statique, déployable dans un conteneur sans bibliothèque système.
# Les tests, eux, ont besoin de cgo pour le détecteur de compétition.
build: export CGO_ENABLED := 0
build:
	go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(PKG)

test:
	go test ./... -race -count=1

# Séparé de `test` : la mesure de couverture réclame l'outil covdata, absent
# des chaînes Go téléchargées automatiquement par GOTOOLCHAIN.
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

lint: fmt vet

fmt:
	@test -z "$$(gofmt -l cmd internal)" || (echo "gofmt requis sur :"; gofmt -l cmd internal; exit 1)

vet:
	go vet ./...

run:
	go run $(PKG)

clean:
	rm -rf bin dist coverage.out

# Binaires de release pour les deux architectures visées par le LXC.
dist: export CGO_ENABLED := 0
dist: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-amd64 $(PKG)
	GOOS=linux GOARCH=arm64 go build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o dist/$(BINARY)-linux-arm64 $(PKG)
	cd dist && sha256sum $(BINARY)-linux-* > SHA256SUMS
