.PHONY: build test vet
BINARY=plugin
VERSION ?= 0.1.6
GO ?= go
build:
	$(GO) build -buildvcs=false -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o $(BINARY) .
test:
	$(GO) test -race ./...
vet:
	$(GO) vet ./...
