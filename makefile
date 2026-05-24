# Makefile for vmxtool - works on macOS, Linux, and Windows (with make installed)

# Version information
VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || powershell -Command "Get-Date -Format yyyy-MM-ddTHH:mm:ssZ")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildDate=$(BUILD_DATE) -X main.Commit=$(COMMIT)

# Directories
BUILD_DIR := build
DIST_DIR := dist

# Detect OS for platform-specific commands
ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
    RM := del /Q /F
    RMDIR := rmdir /S /Q
    MKDIR := mkdir
    CP := copy
    SEP := \\
else
    DETECTED_OS := $(shell uname -s)
    RM := rm -f
    RMDIR := rm -rf
    MKDIR := mkdir -p
    CP := cp
    SEP := /
endif

# Targets
.PHONY: all clean test build build-all dist help version

# Default target
all: test build-all dist

# Help target
help:
	@echo "vmxtool build system"
	@echo ""
	@echo "Targets:"
	@echo "  make              - Run tests, build all platforms, create distribution"
	@echo "  make build        - Build for current platform only"
	@echo "  make build-all    - Build for all platforms"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make dist         - Create distribution archive"
	@echo "  make version      - Show version information"
	@echo ""
	@echo "Version: $(VERSION)"
	@echo "OS: $(DETECTED_OS)"

# Show version
version:
	@echo "Version:    $(VERSION)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Commit:     $(COMMIT)"

# Run tests
test:
	@echo "Running tests..."
	go test -v

# Build for current platform
build:
	@echo "Building for current platform..."
	go build -ldflags="$(LDFLAGS)" -o vmxtool$(if $(filter Windows,$(DETECTED_OS)),.exe,)

# Build for all platforms
build-all: build-windows build-linux build-darwin
	@echo "Copying documentation..."
	$(MKDIR) $(BUILD_DIR)
	$(CP) README.md $(BUILD_DIR)$(SEP)
	$(CP) CHANGELOG.md $(BUILD_DIR)$(SEP)
	$(CP) LICENSE $(BUILD_DIR)$(SEP)
	$(CP) sample.vmx $(BUILD_DIR)$(SEP)
	@echo "All builds complete!"

# Windows builds
build-windows:
	@echo "Building Windows AMD64..."
	$(MKDIR) $(BUILD_DIR)$(SEP)windows$(SEP)amd64
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)windows$(SEP)amd64$(SEP)vmxtool.exe
	@echo "Building Windows ARM64..."
	$(MKDIR) $(BUILD_DIR)$(SEP)windows$(SEP)arm64
	GOOS=windows GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)windows$(SEP)arm64$(SEP)vmxtool.exe

# Linux builds
build-linux:
	@echo "Building Linux AMD64..."
	$(MKDIR) $(BUILD_DIR)$(SEP)linux$(SEP)amd64
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)linux$(SEP)amd64$(SEP)vmxtool
	@echo "Building Linux ARM64..."
	$(MKDIR) $(BUILD_DIR)$(SEP)linux$(SEP)arm64
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)linux$(SEP)arm64$(SEP)vmxtool

# macOS/Darwin builds
build-darwin:
	@echo "Building macOS AMD64..."
	$(MKDIR) $(BUILD_DIR)$(SEP)darwin$(SEP)amd64
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)darwin$(SEP)amd64$(SEP)vmxtool
	@echo "Building macOS ARM64 (Apple Silicon)..."
	$(MKDIR) $(BUILD_DIR)$(SEP)darwin$(SEP)arm64
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)$(SEP)darwin$(SEP)arm64$(SEP)vmxtool

# Create distribution archive
dist:
	@echo "Creating distribution archive..."
	$(MKDIR) $(DIST_DIR)
ifeq ($(DETECTED_OS),Windows)
	@echo "Using PowerShell to create zip..."
	powershell -Command "Compress-Archive -Force -Path $(BUILD_DIR)\* -DestinationPath $(DIST_DIR)\vmxtool-$(VERSION).zip"
	powershell -Command "Get-FileHash -Algorithm SHA256 $(DIST_DIR)\vmxtool-$(VERSION).zip | Select-Object -ExpandProperty Hash | Out-File -Encoding ASCII $(DIST_DIR)\vmxtool-$(VERSION).sha256"
else
	@echo "Creating zip archive..."
	cd $(BUILD_DIR) && zip -r ../$(DIST_DIR)/vmxtool-$(VERSION).zip .
	shasum -a 256 $(DIST_DIR)/vmxtool-$(VERSION).zip > $(DIST_DIR)/vmxtool-$(VERSION).sha256
endif
	@echo "Distribution created: $(DIST_DIR)/vmxtool-$(VERSION).zip"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
ifeq ($(DETECTED_OS),Windows)
	-$(RMDIR) $(BUILD_DIR) 2>nul
	-$(RMDIR) $(DIST_DIR) 2>nul
	-$(RM) vmxtool.exe 2>nul
else
	$(RMDIR) $(BUILD_DIR)
	$(RMDIR) $(DIST_DIR)
	$(RM) vmxtool
endif
	@echo "Clean complete!"
