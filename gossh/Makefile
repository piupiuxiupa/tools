.PHONY: build build-all build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 clean test run

BINARY=gossh
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/gossh

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 ./cmd/gossh

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-linux-arm64 ./cmd/gossh

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-darwin-amd64 ./cmd/gossh

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64 ./cmd/gossh

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-windows-amd64.exe ./cmd/gossh

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64
	rm -rf ./dist
	mkdir -p ./dist
	cp $(BINARY)-linux-amd64 ./dist/
	cp $(BINARY)-linux-arm64 ./dist/
	cp $(BINARY)-darwin-amd64 ./dist/
	cp $(BINARY)-darwin-arm64 ./dist/
	cp $(BINARY)-windows-amd64.exe ./dist/
	@echo "All binaries built and copied to ./dist/"

run:
	go run ./cmd/gossh

test:
	go test -v ./...

clean:
	rm -f $(BINARY)
	rm -f $(BINARY)-linux-amd64
	rm -f $(BINARY)-linux-arm64
	rm -f $(BINARY)-darwin-amd64
	rm -f $(BINARY)-darwin-arm64
	rm -f $(BINARY)-windows-amd64.exe
	rm -rf ./dist

deps:
	go mod tidy
	go mod download