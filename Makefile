.PHONY: build test runtimes releases
build:
	GOOS=linux GOARCH=amd64 go build -o alca-linux ./cmd/alca
	GOOS=windows GOARCH=amd64 go build -o alca-win32.exe ./cmd/alca
test:
	go test ./...
# Runtimes for cross-compilation: ./alca build app.al --target windows/amd64
runtimes:
	go run ./cmd/alca build-runtimes --out runtimes

releases:
	./scripts/make-release.sh
	powershell ./scripts/make-release.ps1
