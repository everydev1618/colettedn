.PHONY: build run dev clean build-lambda test

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

# Run tests (requires ANTHROPIC_API_KEY for smoke tests)
test:
	go test -v ./...

# Deploy to AWS
deploy:
	sam build
	sam deploy
