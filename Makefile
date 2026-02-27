.PHONY: build install test test-integration lint clean

BINARY := sc
PKG := github.com/zpdzap/sandcastles/cmd/sc

build:
	go build -o bin/$(BINARY) $(PKG)

install:
	go install $(PKG)

test:
	go test ./... -count=1

test-integration:
	SANDCASTLES_DOCKER=1 go test ./... -count=1 -timeout 120s

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
