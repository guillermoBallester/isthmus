.PHONY: build test test-short lint fmt vet tidy clean

# Load .env file if it exists.
-include .env
export

build:
	go build -o bin/isthmus ./cmd/isthmus

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
