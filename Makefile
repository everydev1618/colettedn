.PHONY: build run dev clean

build:
	go build -o bin/api ./cmd/api

run: build
	./bin/api

dev:
	go run ./cmd/api

clean:
	rm -rf bin/
