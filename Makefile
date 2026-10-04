# 跨平台 Makefile：按运行平台自动适配
# - Linux：保持标准 POSIX 语法（GOOS 前缀 / cp / rm -f）
# - Windows（PowerShell / cmd / Git Bash 调用）：自动改用 cmd 语法（set / copy / del）
.PHONY: all build linux windows clean dev deploy deploy-server

# Binary names
BINARY_LINUX=bug
BINARY_WINDOWS=bug.exe

# Source file
SOURCE=main.go

# Local install dir for `make deploy` (Windows PATH bin, override with LOCAL_BIN_DIR=...)
LOCAL_BIN_DIR ?= C:/apps/tools

# ---- 平台自动适配：Windows 用 cmd 语法，Linux 用 POSIX 语法 ----
# Windows（含 PowerShell / cmd / Git Bash 调用）统一强制 cmd.exe 执行 recipe；
# Linux 上保持标准 POSIX 命令不变。
ifeq ($(OS),Windows_NT)
  SHELL := cmd.exe
  GO_BUILD_WIN := set "GOOS=windows" && set "GOARCH=amd64" && go build
  GO_BUILD_LIN := set "GOOS=linux" && set "GOARCH=amd64" && go build
  HOST_BIN     := $(BINARY_WINDOWS)
  RM := del /Q
  CP := copy /Y
  ECHO_BLANK := echo.
  # cmd 的 copy 只认反斜杠路径
  DEPLOY_DEST := $(subst /,\,$(LOCAL_BIN_DIR)/$(BINARY_WINDOWS))
else
  GO_BUILD_WIN := GOOS=windows GOARCH=amd64 go build
  GO_BUILD_LIN := GOOS=linux GOARCH=amd64 go build
  HOST_BIN     := $(BINARY_LINUX)
  RM := rm -f
  CP := cp
  ECHO_BLANK := echo
  DEPLOY_DEST := $(LOCAL_BIN_DIR)/$(BINARY_WINDOWS)
endif

# Default target
all: build

# Build for both platforms
build: linux windows

# Build for Linux (amd64)
linux:
	$(GO_BUILD_LIN) -o $(BINARY_LINUX) $(SOURCE)
	@echo "Build complete for Linux: $(BINARY_LINUX)"

# Build for Windows (amd64)
windows:
	$(GO_BUILD_WIN) -o $(BINARY_WINDOWS) $(SOURCE)
	@echo "Build complete for Windows: $(BINARY_WINDOWS)"

# Build for current host platform (quick build)
dev:
	go build -o $(HOST_BIN) $(SOURCE)
	@echo "Build complete for current host: $(HOST_BIN)"

# Local deploy: build Windows binary and install into PATH bin
deploy: windows
	@echo "Installing to $(DEPLOY_DEST)..."
	$(CP) $(BINARY_WINDOWS) $(DEPLOY_DEST)
	@echo "Deploy complete. $(BINARY_WINDOWS) installed to $(DEPLOY_DEST)"

# Deploy Linux binary to remote server
deploy-server: linux
	@echo "Uploading to oo server..."
	scp $(BINARY_LINUX) oo:/usr/local/bin/
	@echo "Deploy complete. Binary uploaded to oo:/usr/local/bin/$(BINARY_LINUX)"
	$(ECHO_BLANK)
	@echo "Next steps (ssh oo):"
	@echo "  sudo mkdir -p /var/lib/bug"
	@echo "  sudo bug install"

# Clean built binaries
clean:
	-$(RM) $(BINARY_LINUX)
	-$(RM) $(BINARY_WINDOWS)
	@echo "Cleaned build files."
