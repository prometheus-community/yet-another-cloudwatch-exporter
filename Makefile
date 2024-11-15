.DEFAULT_GOAL := build

GIT_BRANCH   ?= $(shell git rev-parse --abbrev-ref HEAD)
GIT_REVISION ?= $(shell git rev-parse --short HEAD)
VERSION      ?= $(GIT_BRANCH)-$(GIT_REVISION)
GO_LDFLAGS   := -X main.version=${VERSION}

build:
	go build -v -ldflags '$(GO_LDFLAGS) -w -s -extldflags "-static"' -a -o yace ./cmd/yace

test:
	go test -v -bench=^$$ -race -count=1 ./...

lint:
	golangci-lint run -v -c .golangci.yml
