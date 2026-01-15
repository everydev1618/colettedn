.PHONY: build run dev clean build-lambda

build:
	go build -o bin/api ./cmd/api

run: build
	./bin/api

dev:
	go run ./cmd/api

clean:
	rm -rf bin/ .aws-sam/

# Lambda build (used by SAM)
build-lambda:
	cp -r frontend cmd/lambda/
	cd cmd/lambda && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags "lambda,lambda.norpc" -o bootstrap .
	mv cmd/lambda/bootstrap $(ARTIFACTS_DIR)/
	rm -rf cmd/lambda/frontend

# SAM local testing
sam-local: build-lambda
	sam local start-api

# Deploy to AWS
deploy:
	sam build
	sam deploy
