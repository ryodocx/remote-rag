BINARY_NAME=rrag-bridge
VERSION=1.0.0

.PHONY: build clean test build-all

build:
	go build -o $(BINARY_NAME) .

clean:
	go clean
	rm -f $(BINARY_NAME)
	rm -rf dist/

build-all: clean
	mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -o dist/$(BINARY_NAME) .
	tar -czf dist/$(BINARY_NAME)-darwin-arm64.tar.gz -C dist $(BINARY_NAME)

	GOOS=linux GOARCH=amd64 go build -o dist/$(BINARY_NAME) .
	tar -czf dist/$(BINARY_NAME)-linux-amd64.tar.gz -C dist $(BINARY_NAME)

	GOOS=windows GOARCH=amd64 go build -o dist/$(BINARY_NAME).exe .
	zip -j dist/$(BINARY_NAME)-windows-amd64.zip dist/$(BINARY_NAME).exe

	cd dist && sha256sum *.tar.gz *.zip > checksums.txt
	rm -f dist/$(BINARY_NAME) dist/$(BINARY_NAME).exe

test:
	go test ./...
