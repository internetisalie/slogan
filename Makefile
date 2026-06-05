GO_BIN ?= $(shell which go 2>/dev/null || echo /home/mini/go/go1.26.2/bin/go)

.PHONY: test
test:
	$(GO_BIN) test -v -race ./...
