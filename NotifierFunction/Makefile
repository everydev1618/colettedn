# This Makefile is used by SAM build.

.PHONY: build build-NotifierFunction

# Get project root
PROJECT_ROOT := $(shell git -C $(dir $(realpath $(firstword $(MAKEFILE_LIST)))) rev-parse --show-toplevel 2>/dev/null || echo "$(dir $(realpath $(firstword $(MAKEFILE_LIST))))/..")

build build-NotifierFunction:
	@echo "Building NotifierFunction..."
	cd "$(PROJECT_ROOT)/cmd/notifier" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags "lambda,lambda.norpc" -o $(ARTIFACTS_DIR)/bootstrap .
