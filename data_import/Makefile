# Go 项目 Makefile
# 支持多平台交叉编译

# 项目配置
BINARY_NAME := data_import
MAIN_PACKAGE := ./cmd/data_import
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# 构建参数
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -s -w"

# 目标平台定义
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm/v7 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

# 构建目录
BUILD_DIR := bin
DIST_DIR := dist

# 默认目标
.PHONY: all
all: clean build

# 显示帮助信息
.PHONY: help
help:
	@echo "可用的构建目标:"
	@echo "  make build         - 为当前平台构建"
	@echo "  make build-all     - 为所有支持的平台构建"
	@echo "  make build-linux   - 为 Linux 平台构建 (amd64, arm64, arm/v7)"
	@echo "  make build-darwin  - 为 macOS 平台构建 (amd64, arm64)"
	@echo "  make build-windows - 为 Windows 平台构建 (amd64, arm64)"
	@echo "  make clean         - 清理构建产物"
	@echo "  make package       - 打包所有构建产物"
	@echo "  make install       - 安装到系统"
	@echo ""
	@echo "变量:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

# 为当前平台构建
.PHONY: build
build:
	@echo "Building for current platform..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_NAME)"

# 为所有平台构建
.PHONY: build-all
build-all: build-linux build-darwin build-windows
	@echo "All platforms built successfully!"

# Linux 平台构建
.PHONY: build-linux
build-linux: build-linux-amd64 build-linux-arm64 build-linux-armv7

.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

.PHONY: build-linux-armv7
build-linux-armv7:
	@echo "Building for linux/arm/v7..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7 $(MAIN_PACKAGE)

# macOS 平台构建
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for darwin/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for darwin/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# Windows 平台构建
.PHONY: build-windows
build-windows: build-windows-amd64 build-windows-arm64

.PHONY: build-windows-amd64
build-windows-amd64:
	@echo "Building for windows/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

.PHONY: build-windows-arm64
build-windows-arm64:
	@echo "Building for windows/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PACKAGE)

# 使用 xgo 构建所有平台 (需要安装 xgo)
.PHONY: xgo-build
xgo-build:
	@if command -v xgo >/dev/null 2>&1; then \
		echo "Building with xgo..."; \
		xgo -targets=linux/amd64,linux/arm64,darwin/amd64,darwin/arm64,windows/amd64 \
			-ldflags "$(LDFLAGS)" \
			-o $(BUILD_DIR)/$(BINARY_NAME) \
			$(MAIN_PACKAGE); \
	else \
		echo "xgo not installed. Install with: go install github.com/crazy-max/xgo@latest"; \
		exit 1; \
	fi

# 清理构建产物
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR) $(BINARY_NAME)
	@echo "Clean complete!"

# 创建发布包
.PHONY: package
package: build-all
	@echo "Creating distribution packages..."
	@mkdir -p $(DIST_DIR)
	@for file in $(BUILD_DIR)/*; do \
		if [ -f "$$file" ]; then \
			filename=$$(basename "$$file"); \
			tar -czf "$(DIST_DIR)/$$filename.tar.gz" -C $(BUILD_DIR) "$$filename"; \
			echo "  Packaged: $$filename.tar.gz"; \
		fi \
	done
	@echo "Packages created in $(DIST_DIR)/"

# 安装到系统
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "Install complete!"

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# 代码格式化
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

# 代码检查
.PHONY: lint
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping..."; \
	fi

# 依赖整理
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# 版本信息
.PHONY: version
version:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(shell go version)"
