# This Makefile is used by SAM build.
# Prerequisites: Run 'npm run build' from project root first.
# The GitHub Actions workflow handles this automatically for deployment.

.PHONY: build build-ColetteDNFunction

# Get project root - SAM runs this from a temp copy of ColetteDNFunction
# so we need to find the real project root
PROJECT_ROOT := $(shell git -C $(dir $(realpath $(firstword $(MAKEFILE_LIST)))) rev-parse --show-toplevel 2>/dev/null || echo "$(dir $(realpath $(firstword $(MAKEFILE_LIST))))/..")

build build-ColetteDNFunction:
	@echo "Building ColetteDNFunction..."
	@echo "Project root: $(PROJECT_ROOT)"
	@if [ ! -d "$(PROJECT_ROOT)/frontend/static/dist" ]; then \
		echo "Error: frontend/static/dist not found. Run 'npm run build' from project root first."; \
		exit 1; \
	fi
	cp -r "$(PROJECT_ROOT)/frontend" $(ARTIFACTS_DIR)/
	cd "$(PROJECT_ROOT)/cmd/lambda" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags "lambda,lambda.norpc" -o $(ARTIFACTS_DIR)/bootstrap .
