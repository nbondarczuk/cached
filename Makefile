all: test benchmark

test:
	go test -v

benchmark:
	go test -bench=.

lint: install
	gofmt -s -w .
	go vet .
	golint .
	staticcheck .
	gocyclo -over 15 .

install:
	go install golang.org/x/lint/golint@latest
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

help:
	@echo "Available targets:"
	@echo "  all        - Run tests and benchmarks"
	@echo "  test       - Run tests"
	@echo "  benchmark  - Run benchmarks"
	@echo "  lint       - Run linters"
	@echo "  install    - Install golint"
	@echo "  help       - Show this help message"

.PHONY: all test benchmark help lint install