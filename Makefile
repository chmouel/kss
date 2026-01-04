NAMES = kss tkss

all: build

mkdir:
	mkdir -p bin

build: mkdir
	for name in $(NAMES); do \
		go build -o bin/$$name ./cmd/$$name; \
	done

sanity: lint format test

lint:
	golangci-lint run --fix ./...

format:
	gofumpt -w .

test:
	go test -v ./...

coverage:
	go test ./... -covermode=count -coverprofile=coverage.out
	grep -vE "cmd/|internal/ai/|internal/kube/|internal/ui/" coverage.out > coverage.filtered.out
	go tool cover -func=coverage.filtered.out
	rm coverage.filtered.out


completions:
	@mkdir -p completions
	for name in $(NAMES); do \
		go run ./cmd/$$name --completion bash > completions/$$name.bash; \
		go run ./cmd/$$name --completion zsh > completions/$$name.zsh; \
	done

.PHONY: all build lint format test coverage sanity mkdir completions
