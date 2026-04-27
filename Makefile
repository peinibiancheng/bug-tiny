.PHONY: all build linux windows clean dev

# Binary names
BINARY_LINUX=bug
BINARY_WINDOWS=bug.exe

# Source file
SOURCE=main.go

# Default target
all: build

# Build for both platforms
build: linux windows

# Build for Linux (amd64)
linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_LINUX) $(SOURCE)
	@echo "Build complete for Linux: $(BINARY_LINUX)"

# Build for Windows (amd64)
windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_WINDOWS) $(SOURCE)
	@echo "Build complete for Windows: $(BINARY_WINDOWS)"

# Build for current host platform (quick build)
dev:
	go build -o $(BINARY_LINUX) $(SOURCE)
	@echo "Build complete for current host."

# Clean built binaries
clean:
	rm -f $(BINARY_LINUX)
	rm -f $(BINARY_WINDOWS)
	@echo "Cleaned build files."
