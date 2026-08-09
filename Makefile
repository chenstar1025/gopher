.PHONY: test build build-all vet fmt clean

# Run all tests
test:
	go test ./... -v -count=1

# Run tests with race detector
test-race:
	go test ./... -v -race -count=1

# Run tests with coverage
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Build for current platform
build:
	go build -o gopher ./cmd/gopher/

# Cross-compile for all target platforms
build-all:
	GOOS=windows GOARCH=amd64 go build -o dist/gopher-windows-amd64.exe ./cmd/gopher/
	GOOS=linux   GOARCH=amd64 go build -o dist/gopher-linux-amd64    ./cmd/gopher/
	GOOS=darwin  GOARCH=amd64 go build -o dist/gopher-darwin-amd64   ./cmd/gopher/
	GOOS=darwin  GOARCH=arm64 go build -o dist/gopher-darwin-arm64   ./cmd/gopher/

vet:
	go vet ./...

fmt:
	gofmt -l .

clean:
	rm -rf dist/ gopher gopher.exe coverage.out coverage.html
