GO ?= go
BIN := bin/bp-tracker

.PHONY: all web build test test-go test-web dev-server dev-web clean

all: build

web:
	cd web && npm ci && npm run build
	touch web/dist/.gitkeep

build: web
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o $(BIN) ./cmd/server

test: test-go test-web

test-go:
	$(GO) test ./...

test-web:
	cd web && npm run test

dev-server:
	$(GO) run ./cmd/server -db ./data/bp.db -addr :8080

dev-web:
	cd web && npm run dev

clean:
	rm -rf bin
	find web/dist -mindepth 1 ! -name .gitkeep -delete
