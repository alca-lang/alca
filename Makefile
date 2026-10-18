.PHONY: build test runtimes
build:
	go build -o alca ./cmd/alca
test:
	go test ./...
# Runtimes for cross-compilation: ./alca build app.al --target windows/amd64
runtimes:
	go run ./cmd/alca build-runtimes --out runtimes
