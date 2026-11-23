# Copyright 2026 Pawl Programming Language
# SPDX-License-Identifier: Apache-2.0

.PHONY: build test runtimes releases
build:
	GOOS=linux GOARCH=amd64 go build -o pawl-linux ./cmd/pawl
	GOOS=windows GOARCH=amd64 go build -o pawl-win32.exe ./cmd/pawl
test:
	go test ./...
# Runtimes for cross-compilation: ./pawl build app.al --target windows/amd64
runtimes:
	go run ./cmd/pawl build-runtimes --out runtimes

releases:
	./scripts/make-release.sh
	powershell ./scripts/make-release.ps1
