.PHONY: build install test clean lint lint-fix fmt vet staticcheck check tidy help

BINARY  := pi-stream
PKG     := github.com/crazy-goat/pi-stream/internal/cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION)

GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo "$$(go env GOPATH)/bin/golangci-lint")

help:
	@echo "Available targets:"
	@echo "  make build        - Build the binary (with version embedded)"
	@echo "  make install      - Build and copy binary to \$$HOME/.local/bin"
	@echo "  make test         - Run tests with -race"
	@echo "  make lint         - Run golangci-lint"
	@echo "  make lint-fix     - Run golangci-lint with auto-fix"
	@echo "  make fmt          - Format code with gofmt"
	@echo "  make vet          - Run go vet"
	@echo "  make staticcheck  - Run staticcheck"
	@echo "  make tidy         - go mod tidy + diff check"
	@echo "  make check        - Run fmt, vet, staticcheck, lint, test, build"
	@echo "  make clean        - Remove built binary"

build:
	go build -ldflags='$(LDFLAGS)' -o $(BINARY) .

install: build
	install -d $(HOME)/.local/bin
	install -m 0755 $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "Installed $(BINARY) to $(HOME)/.local/bin"

test:
	go test -race -count=1 ./...

fmt:
	@echo "Formatting code..."
	@gofmt -w .
	@echo "Done"

vet:
	@echo "Running go vet..."
	@go vet ./...
	@echo "Done"

lint:
	@echo "Running golangci-lint..."
	@if [ -x "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
		exit 1; \
	fi

lint-fix:
	@echo "Running golangci-lint with auto-fix..."
	@if [ -x "$(GOLANGCI_LINT)" ]; then \
		$(GOLANGCI_LINT) run --fix ./...; \
	else \
		echo "golangci-lint not installed."; \
		exit 1; \
	fi

staticcheck:
	@echo "Running staticcheck..."
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "staticcheck not installed. Install with:"; \
		echo "  go install honnef.co/go/tools/cmd/staticcheck@latest"; \
		exit 1; \
	fi

tidy:
	go mod tidy
	@git diff --exit-code -- go.mod go.sum || (echo "go.mod/go.sum changed; commit the tidy result"; exit 1)

check: fmt vet staticcheck lint test build
	@echo "All checks passed!"

clean:
	rm -f $(BINARY)
