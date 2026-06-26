BINARY_NAME=remote-rag-bridge
VERSION=1.0.0

.PHONY: build clean test build-all

build:
	go build -o $(BINARY_NAME) .

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-darwin-amd64
	rm -f $(BINARY_NAME)-darwin-arm64
	rm -f $(BINARY_NAME)-linux-amd64
	rm -f $(BINARY_NAME)-windows-amd64.exe

build-all:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 .
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_NAME)-windows-amd64.exe .

test:
	go test ./...
