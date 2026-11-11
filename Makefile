.PHONY: build test test-e2e run

build:
	cd web && npm ci && npm run build
	CGO_ENABLED=0 go build -o bin/agent-desk .

test: build
	go test -race ./...
	go vet ./...

test-e2e: build
	cd web && npm run test:e2e

run:
	./bin/agent-desk
