.PHONY: help tidy build verify test test-coverage test-verbose clean install lint fmt vet clean-cache

# Default target
help:
	@echo "Available targets:"
	@echo "  tidy           - Sync and clean Go module dependencies"
	@echo "  build          - Build the kubectl-flex-to-csi binary"
	@echo "  verify         - Run go mod tidy and full project build"
	@echo "  test           - Run all tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  clean          - Remove build artifacts"
	@echo "  clean-cache    - Clear Go build and gopls cache (fixes IDE errors)"
	@echo "  install        - Install the binary to /usr/local/bin"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format code with gofmt"
	@echo "  vet            - Run go vet"
	@echo "  dist           - Create all distribution packages (macOS, Windows, Linux)"
	@echo "  dist-mac       - Create macOS distribution package"
	@echo "  dist-windows   - Create Windows distribution package"
	@echo "  dist-linux     - Create Linux distribution package"
	@echo "  dist-clean     - Remove distribution packages"

# Sync Go modules
tidy:
	@echo "Tidying Go modules..."
	go mod tidy
	@echo "✅ Go modules are tidy"

# Build the binary
build:
	@echo "Building kubectl-flex-to-csi..."
	go build -o kubectl-flex-to-csi ./cmd/kubectl-flex-to-csi/main.go
	@echo "✅ Build complete"

# Verify module state and full package build
verify:
	@echo "Verifying module state and project build..."
	go mod tidy
	go build ./...
	@echo "✅ Verification complete"

# Run all tests
test:
	@echo "Running tests..."
	go test -v -race ./...
	@echo "✅ Tests complete"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total
	@echo "✅ Coverage report generated: coverage.html"

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "✅ Tests complete"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f kubectl-flex-to-csi
	rm -f coverage.out coverage.html
	@echo "✅ Clean complete"

# Clear Go build and gopls cache (fixes IDE false positive errors)
clean-cache:
	@echo "Clearing Go build cache..."
	go clean -cache -modcache -testcache
	@echo "Clearing gopls cache..."
	@rm -rf ~/Library/Caches/go-build 2>/dev/null || true
	@rm -rf ~/.cache/gopls 2>/dev/null || true
	@rm -rf ~/Library/Caches/gopls 2>/dev/null || true
	@echo "Running go mod tidy..."
	go mod tidy
	@echo "Rebuilding to refresh cache..."
	go build ./...
	@echo "✅ Cache cleared - restart your IDE/editor for changes to take effect"

# Install binary
install: build
	@echo "Installing kubectl-flex-to-csi..."
	sudo cp kubectl-flex-to-csi /usr/local/bin/kubectl-flex-to-csi
	@echo "✅ Installation complete"

# Run linter (requires golangci-lint)
lint:
	@echo "Running golangci-lint..."
	golangci-lint run ./...
	@echo "✅ Lint complete"

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	@echo "✅ Format complete"

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...
	@echo "✅ Vet complete"

# Run all checks
check: fmt vet test
	@echo "✅ All checks passed"

# Create distribution packages
dist: dist-mac dist-windows dist-linux
	@echo "✅ All distribution packages created"

# Create macOS distribution
dist-mac:
	@echo "Creating macOS distribution package..."
	@echo "Building macOS binary with optimized flags..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o kubectl-flex-to-csi-amd64 ./cmd/kubectl-flex-to-csi/main.go
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o kubectl-flex-to-csi-arm64 ./cmd/kubectl-flex-to-csi/main.go
	@echo "Creating universal binary..."
	@lipo -create -output kubectl-flex-to-csi kubectl-flex-to-csi-amd64 kubectl-flex-to-csi-arm64 || cp kubectl-flex-to-csi-amd64 kubectl-flex-to-csi
	@rm -f kubectl-flex-to-csi-amd64 kubectl-flex-to-csi-arm64
	@rm -rf dist/kubectl-flex-to-csi-macos
	@mkdir -p dist/kubectl-flex-to-csi-macos/kubeconfig
	@cp kubectl-flex-to-csi dist/kubectl-flex-to-csi-macos/
	@cp run.command dist/kubectl-flex-to-csi-macos/
	@cp LAUNCHER_README.md dist/kubectl-flex-to-csi-macos/
	@cp MACOS_INSTALL.md dist/kubectl-flex-to-csi-macos/
	@cp dist-template/kubeconfig/README.md dist/kubectl-flex-to-csi-macos/kubeconfig/
	@echo "#!/bin/bash" > dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "# Remove quarantine attribute from macOS" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "xattr -d com.apple.quarantine kubectl-flex-to-csi 2>/dev/null || true" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "xattr -d com.apple.quarantine run.command 2>/dev/null || true" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "chmod +x kubectl-flex-to-csi" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "chmod +x run.command" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@echo "echo 'Installation complete! You can now run ./kubectl-flex-to-csi or ./run.command'" >> dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@chmod +x dist/kubectl-flex-to-csi-macos/INSTALL.sh
	@chmod +x dist/kubectl-flex-to-csi-macos/run.command
	@chmod +x dist/kubectl-flex-to-csi-macos/kubectl-flex-to-csi
	@cd dist && zip -r kubectl-flex-to-csi-macos.zip kubectl-flex-to-csi-macos
	@rm kubectl-flex-to-csi
	@echo "✅ macOS package created: dist/kubectl-flex-to-csi-macos.zip"

# Create Windows distribution
dist-windows:
	@echo "Creating Windows distribution package..."
	@echo "Building Windows binary..."
	@GOOS=windows GOARCH=amd64 go build -o kubectl-flex-to-csi.exe ./cmd/kubectl-flex-to-csi/main.go
	@rm -rf dist/kubectl-flex-to-csi-windows
	@mkdir -p dist/kubectl-flex-to-csi-windows/kubeconfig
	@cp kubectl-flex-to-csi.exe dist/kubectl-flex-to-csi-windows/
	@cp run.bat dist/kubectl-flex-to-csi-windows/
	@cp LAUNCHER_README.md dist/kubectl-flex-to-csi-windows/
	@cp dist-template/kubeconfig/README.md dist/kubectl-flex-to-csi-windows/kubeconfig/
	@cd dist && zip -r kubectl-flex-to-csi-windows.zip kubectl-flex-to-csi-windows
	@rm kubectl-flex-to-csi.exe
	@echo "✅ Windows package created: dist/kubectl-flex-to-csi-windows.zip"

# Create Linux distribution
dist-linux:
	@echo "Creating Linux distribution package..."
	@echo "Building Linux binary..."
	@GOOS=linux GOARCH=amd64 go build -o kubectl-flex-to-csi-linux ./cmd/kubectl-flex-to-csi/main.go
	@rm -rf dist/kubectl-flex-to-csi-linux
	@mkdir -p dist/kubectl-flex-to-csi-linux/kubeconfig
	@cp kubectl-flex-to-csi-linux dist/kubectl-flex-to-csi-linux/kubectl-flex-to-csi
	@cp run.command dist/kubectl-flex-to-csi-linux/run.sh
	@cp LAUNCHER_README.md dist/kubectl-flex-to-csi-linux/
	@cp dist-template/kubeconfig/README.md dist/kubectl-flex-to-csi-linux/kubeconfig/
	@chmod +x dist/kubectl-flex-to-csi-linux/run.sh
	@chmod +x dist/kubectl-flex-to-csi-linux/kubectl-flex-to-csi
	@cd dist && tar -czf kubectl-flex-to-csi-linux.tar.gz kubectl-flex-to-csi-linux
	@rm kubectl-flex-to-csi-linux
	@echo "✅ Linux package created: dist/kubectl-flex-to-csi-linux.tar.gz"

# Clean distribution packages
dist-clean:
	@echo "Cleaning distribution packages..."
	@rm -rf dist
	@echo "✅ Distribution packages cleaned"