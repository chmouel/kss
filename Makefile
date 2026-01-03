NAME = kss

all: build

mkdir:
	mkdir -p bin

build: mkdir
	go build -o bin/$(NAME) ./cmd/$(NAME)

sanity: lint format test

lint:
	golangci-lint run --fix ./...

format:
	gofumpt -w .

test:
	go test -v ./...

coverage:
	go test ./... -covermode=count -coverprofile=coverage.out
	go tool cover -func=coverage.out

.PHONY: all build lint format test coverage sanity mkdir
