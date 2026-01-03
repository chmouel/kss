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
	grep -vE "cmd/|internal/ai/" coverage.out > coverage.filtered.out
	go tool cover -func=coverage.filtered.out
	rm coverage.filtered.out


completions:
	@mkdir -p completions
	go run ./cmd/kss --completion bash > completions/kss.bash
	go run ./cmd/kss --completion zsh > completions/kss.zsh

.PHONY: all build lint format test coverage sanity mkdir completions
