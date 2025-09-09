# Makefile for gh-aca-utils

.PHONY: test build clean lint fmt vet

# Test target
test:
	go test -v ./...

# Build target
build:
	go build -o gh-aca-utils .

# Clean target
clean:
	rm -f gh-aca-utils
	go clean

# Lint target
lint:
	golangci-lint run

# Format target
fmt:
	go fmt ./...

# Vet target
vet:
	go vet ./...

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run all checks
check: fmt vet lint test

# Default target
all: deps check build