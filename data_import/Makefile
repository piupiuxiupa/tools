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
	@echo "  make build             - 为当前平台构建 (含 Oracle 支持, 需要 CGO)"
	@echo "  make build-nooracle    - 为当前平台构建 (无 Oracle 支持, CGO 禁用)"
	@echo "  make build-all         - 为所有支持的平台构建 (推荐)"
	@echo "  make build-linux       - 为 Linux 平台构建 (amd64, arm64, arm/v7, 无 Oracle)"
	@echo "  make build-linux-native - 为 Linux 平台构建 (带 Oracle, 需在 Linux 上运行)"
	@echo "  make build-darwin      - 为 macOS 平台构建 (amd64, arm64, 带 Oracle)"
	@echo "  make build-windows     - 为 Windows 平台构建 (amd64, arm64, 无 Oracle)"
	@echo "  make build-windows-oracle - 显示 Windows + Oracle 构建说明"
	@echo "  make clean             - 清理构建产物"
	@echo "  make package           - 打包所有构建产物"
	@echo "  make install           - 安装到系统 (当前平台, 带 Oracle)"
	@echo ""
	@echo "构建说明:"
	@echo "  - Oracle 支持需要 CGO 和 Oracle Instant Client"
	@echo "  - 交叉编译 (Linux/Windows) 默认禁用 CGO，因此不带 Oracle 支持"
	@echo "  - macOS 可以在本地交叉编译，因此支持带 Oracle 的版本"
	@echo ""
	@echo "变量:"
	@echo "  VERSION=$(VERSION)"
	@echo "  BUILD_TIME=$(BUILD_TIME)"

# 为当前平台构建 (含 Oracle 支持, 需要 CGO 和 Oracle 客户端)
.PHONY: build
build:
	@echo "Building for current platform with Oracle support..."
	CGO_ENABLED=1 go build -tags oracle $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_NAME)"

# 构建无 Oracle 支持版本 (交叉编译友好)
.PHONY: build-nooracle
build-nooracle:
	@echo "Building without Oracle support (CGO disabled)..."
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Build complete: $(BINARY_NAME)"

# 为所有平台构建 (macOS 带 Oracle，Linux/Windows 不带 Oracle)
.PHONY: build-all
build-all: build-darwin-amd64 build-darwin-arm64 build-linux-amd64 build-linux-arm64 build-windows-amd64
	@echo "All platforms built successfully!"
	@echo ""
	@echo "构建结果:"
	@echo "  - Darwin (macOS): 带 Oracle 支持"
	@echo "  - Linux: 无 Oracle 支持 (交叉编译)"
	@echo "  - Windows: 无 Oracle 支持 (交叉编译)"

# Linux 平台构建 (无 Oracle 支持，交叉编译友好)
.PHONY: build-linux
build-linux: build-linux-amd64 build-linux-arm64

.PHONY: build-linux-amd64
build-linux-amd64:
	@echo "Building for linux/amd64 (without Oracle support)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

.PHONY: build-linux-arm64
build-linux-arm64:
	@echo "Building for linux/arm64 (without Oracle support)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

# Linux ARM v7 构建
.PHONY: build-linux-armv7
build-linux-armv7:
	@echo "Building for linux/arm/v7 (without Oracle support)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv7 $(MAIN_PACKAGE)

# Linux 原生构建 (带 Oracle 支持，需要在 Linux 上运行)
.PHONY: build-linux-native
build-linux-native:
	@echo "Building for Linux with Oracle support..."
	@echo "Note: This must be run on a Linux system with Oracle Instant Client"
	CGO_ENABLED=1 go build -tags oracle $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-linux-native $(MAIN_PACKAGE)

# macOS 平台构建 (可在 macOS 上本地交叉编译，支持 Oracle)
.PHONY: build-darwin
build-darwin: build-darwin-amd64 build-darwin-arm64

.PHONY: build-darwin-amd64
build-darwin-amd64:
	@echo "Building for darwin/amd64 with Oracle support..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 go build -tags oracle $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)

.PHONY: build-darwin-arm64
build-darwin-arm64:
	@echo "Building for darwin/arm64 with Oracle support..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 go build -tags oracle $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

# Windows 平台构建 (无 Oracle 支持，交叉编译友好)
.PHONY: build-windows
build-windows: build-windows-amd64 build-windows-arm64

.PHONY: build-windows-amd64
build-windows-amd64:
	@echo "Building for windows/amd64 (without Oracle support)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

.PHONY: build-windows-arm64
build-windows-arm64:
	@echo "Building for windows/arm64 (without Oracle support)..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) \
		-o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PACKAGE)

# Windows + Oracle 构建说明
.PHONY: build-windows-oracle
build-windows-oracle:
	@echo "Windows + Oracle 构建说明:"
	@echo ""
	@echo "由于 Windows + Oracle 需要 CGO，无法在 macOS/Linux 上直接交叉编译。"
	@echo ""
	@echo "可选方案:"
	@echo ""
	@echo "1. 在 Windows 上本地构建 (推荐):"
	@echo "   - 安装 Go"
	@echo "   - 安装 Oracle Instant Client for Windows"
	@echo "   - 运行: set CGO_ENABLED=1 && go build -tags oracle -o data_import.exe ./cmd/data_import"
	@echo ""
	@echo "2. 使用 GitHub Actions:"
	@echo "   - 已配置工作流: .github/workflows/build-windows.yml"
	@echo "   - 在 GitHub 上创建 release tag，会自动构建 Windows 版本"
	@echo "   - 或手动触发 Actions 工作流"
	@echo ""
	@echo "3. 使用 Docker + 特殊工具链 (复杂):"
	@echo "   - 需要 mingw-w64 交叉编译器"
	@echo "   - 需要 Windows 版本的 Oracle 头文件和库"
	@echo "   - 参考: https://github.com/godror/godror/issues/303"
	@echo ""
	@exit 1

# 使用 Docker 构建 Linux 版本 (带 Oracle 支持)
.PHONY: docker-build-linux
docker-build-linux:
	@echo "Building Linux binaries with Oracle support using Docker..."
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

# 安装到系统 (当前平台, 带 Oracle 支持)
.PHONY: install
install:
	@echo "Installing $(BINARY_NAME) with Oracle support..."
	CGO_ENABLED=1 go install -tags oracle $(LDFLAGS) $(MAIN_PACKAGE)
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
