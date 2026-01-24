.PHONY: build run dev clean build-lambda test test-unit test-smoke

build:
	go build -o bin/api ./cmd/api

run: build
	./bin/api

dev:
	go run ./cmd/api

clean:
	rm -rf bin/ .aws-sam/

# Lambda build (used by SAM)
build-ColetteDNFunction:
	cp -r frontend $(ARTIFACTS_DIR)/
	cd cmd/lambda && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags "lambda,lambda.norpc" -o $(ARTIFACTS_DIR)/bootstrap .

# SAM local testing
sam-local:
	sam local start-api

# Run all tests (unit + smoke if ANTHROPIC_API_KEY set)
test:
	go test -v ./...

# Run unit tests only (no API calls)
test-unit:
	go test -v ./internal/namecheap/ ./internal/ratelimit/ ./internal/generator/ ./internal/handler/ -skip TestModelSmoke

# Run smoke test (requires ANTHROPIC_API_KEY)
test-smoke:
	go test -v -run TestModelSmoke ./internal/generator/

# Deploy to AWS
deploy:
	sam build
	sam deploy
