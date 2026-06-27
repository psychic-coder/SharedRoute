.PHONY: build test lint run bench clean

build:
	go build -o bin/shardroute ./cmd/shardroute
	go build -o bin/shardroute-bench ./cmd/shardroute-bench

test:
	go test -race ./...

lint:
	golangci-lint run

run: build
	./bin/shardroute

bench: build
	./bin/shardroute-bench -url http://localhost:8080/v1/check

clean:
	rm -rf bin/
