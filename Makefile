.PHONY: test test-all build-all clean

# Run tests on current platform
test:
	go test -v ./...

# Build for all supported platforms to verify compilation
build-all:
	@echo "Building for macOS (darwin/amd64)..."
	GOOS=darwin GOARCH=amd64 go build ./...
	@echo "Building for macOS (darwin/arm64)..."
	GOOS=darwin GOARCH=arm64 go build ./...
	@echo "Building for Windows (windows/amd64)..."
	GOOS=windows GOARCH=amd64 go build ./...
	@echo "Building for Linux (linux/amd64)..."
	GOOS=linux GOARCH=amd64 go build ./...
	@echo "Building for Linux (linux/arm64)..."
	GOOS=linux GOARCH=arm64 go build ./...
	@echo "Building for iOS (ios/arm64)..."
	GOOS=ios GOARCH=arm64 go build ./...
	@echo "Building for Android (android/arm64)..."
	GOOS=android GOARCH=arm64 go build ./...
	@echo "Building for WebAssembly (js/wasm)..."
	GOOS=js GOARCH=wasm go build ./...
	@echo "All platforms built successfully!"

# Test with Docker containers for Linux
test-linux:
	docker run --rm -v $(PWD):/app -w /app golang:latest go test -v ./...

# Test with Wine for Windows (requires wine to be installed)
test-windows:
	@echo "Cross-compilation check for Windows..."
	GOOS=windows GOARCH=amd64 go build -o vault_test.exe ./...
	@echo "Note: To actually run Windows tests, use a Windows VM or Wine"
	@rm -f vault_test.exe

clean:
	rm -f vault_test.exe
	go clean ./...
