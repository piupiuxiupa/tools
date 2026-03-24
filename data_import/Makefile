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
# 格式: OS/ARCH
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
	@echo "  make build         - 为当前平台构建 (含 Oracle 支持)"
	@echo "  make build-all     - 为所有支持的平台构建"
	@echo "  make build-linux   - 为 Linux 平台构建 (amd64, arm64, arm/v7)"
	@echo "  make build-darwin  - 为 macOS 平台构建 (amd64, arm64)"
	@echo "  make build-windows - 为 Windows 平台构建 (amd64, arm64)"
	@echo "  make build-nooracle - 构建无 Oracle 支持版本 (交叉编译友好)"
	@echo "  make clean         - 清理构建产物"
	@echo "  make package       - 打包所有构建产物"
	@echo "  make install       - 安装到系统 (当前平台)"
	@echo ""
	@echo "注意: 由于项目依赖 godror (Oracle 驱动), 默认构建需要:"
	@echo "      - CGO_ENABLED=1"
	@echo "      - Oracle Instant Client (交叉编译时需要相应工具链)"
	@echo ""
	@echo "变量:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

# 为当前平台构建 (含 Oracle 支持, 需要 CGO 和 Oracle 客户端)
.PHONY: build
build:
	@echo "Building for current platform with Oracle support..."
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_NAME)"

# 构建无 Oracle 支持版本 (交叉编译友好)
.PHONY: build-nooracle
build-nooracle:
	@echo "Building without Oracle support..."
	@echo "WARNING: This requires removing godror import from internal/db/manager.go"
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_NAME)"

# 为所有平台构建
.PHONY: build-all
build-all: build-darwin-amd64 build-darwin-arm64
	@echo "All platforms built successfully!"
	@echo "Note: Linux/Windows builds require cross-compilation toolchains with Oracle support"

# Linux 平台构建 (需要 Docker 或交叉编译工具链)
.PHONY: build-linux
build-linux:
	@echo "Linux builds require cross-compilation with CGO (Oracle support)"
	@echo "Options:"
	@echo "  1. Use Docker: docker run --rm -v $(PWD):/src -w /src golang:1.24 make build-linux-native"
	@echo "  2. Use xgo: xgo --targets=linux/* $(LDFLAGS) $(MAIN_PACKAGE)"
	@echo "  3. Build natively on Linux with Oracle Instant Client installed"

# 在 Linux 环境下构建 (需要在 Linux 系统或 Docker 中运行)
.PHONY: build-linux-native
build-linux-native: build-linux-amd64-native build-linux-arm64-native

.PHONY: build-linux-amd64-native
build-linux-amd64-native:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: build-linux-arm64-native
build-linux-arm64-native:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

# macOS 平台构建 (可在 macOS 上本地交叉编译)
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for darwin/amd64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for darwin/arm64..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# Windows 平台构建 (需要 mingw-w64 或交叉编译工具链)
.PHONY: build-windows
build-windows:
	@echo "Windows builds require cross-compilation with CGO (Oracle support)"
	@echo "Options:"
	@echo "  1. Use xgo: xgo --targets=windows/* $(LDFLAGS) $(MAIN_PACKAGE)"
	@echo "  2. Build natively on Windows with Oracle Instant Client installed"

# 使用 Docker 构建 Linux 版本
.PHONY: docker-build-linux
docker-build-linux:
	@echo "Building Linux binaries using Docker..."
	docker run --rm -v $(PWD):/src -w /src golang:1.24-bookworm \
		bash -c "apt-get update && apt-get install -y libaio1 \
		&& make build-linux-native"

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
package: build-darwin
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

# 安装到系统 (仅当前平台, 需要 Oracle 客户端)
.PHONY: install
install: build
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
