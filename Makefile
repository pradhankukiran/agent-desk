.PHONY: build test run
build:
	cd web && npm ci && npm run build
	CGO_ENABLED=0 go build -o bin/agent-desk .
test:
	go test ./...
run:
	./bin/agent-desk
