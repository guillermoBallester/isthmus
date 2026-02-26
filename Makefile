VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build run dry-run test test-short lint fmt vet tidy clean

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o bin/isthmus ./cmd/isthmus

run: build
	@set -a && source .env && set +a && ./bin/isthmus

dry-run: build
	@set -a && source .env && set +a && ./bin/isthmus --dry-run

test:
	go test -race -count=1 ./...

test-short:
	go test -short -race -count=1 ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 || { echo "Install golangci-lint: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
