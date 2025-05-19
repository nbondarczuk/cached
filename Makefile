all: test benchmark

test:
	go test -v

benchmark:
	go test -bench=.

.PHONY: all test benchmark
