# Minecraft 多服务器备份工具 Makefile

# 变量定义
BINARY_NAME=minecraft-backup
GO_FILES=minecraft-backup.go
INSTALL_PATH=/usr/local/bin
USER_BIN_PATH=$(HOME)/.local/bin

# 默认目标
.PHONY: all
all: build

# 编译
.PHONY: build
build:
	@echo "编译 $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) $(GO_FILES)
	@echo "编译完成: ./$(BINARY_NAME)"

# 运行
.PHONY: run
run:
	go run $(GO_FILES)

# 安装到系统（需要 sudo）
.PHONY: install
install: build
	@echo "安装 $(BINARY_NAME) 到 $(INSTALL_PATH)..."
	sudo cp $(BINARY_NAME) $(INSTALL_PATH)/
	sudo chmod +x $(INSTALL_PATH)/$(BINARY_NAME)
	@echo "安装完成: $(INSTALL_PATH)/$(BINARY_NAME)"
	@echo ""
	@# 检查 PATH
	@if echo $$PATH | grep -q "$(INSTALL_PATH)"; then \
		echo "✓ $(INSTALL_PATH) 已在 PATH 中"; \
		echo "  你可以直接运行: $(BINARY_NAME)"; \
	else \
		echo "⚠️  警告: $(INSTALL_PATH) 不在 PATH 中"; \
		echo "  请添加以下内容到你的 shell 配置文件 (~/.bashrc 或 ~/.zshrc):"; \
		echo "  export PATH=\"$(INSTALL_PATH):\$$PATH\""; \
		echo ""; \
		echo "  或者使用完整路径运行: $(INSTALL_PATH)/$(BINARY_NAME)"; \
	fi

# 用户级安装（不需要 sudo）
.PHONY: install-user
install-user: build
	@echo "安装 $(BINARY_NAME) 到用户目录 $(USER_BIN_PATH)..."
	@# 创建目录（如果不存在）
	@mkdir -p $(USER_BIN_PATH)
	@# 复制文件
	cp $(BINARY_NAME) $(USER_BIN_PATH)/
	chmod +x $(USER_BIN_PATH)/$(BINARY_NAME)
	@echo "安装完成: $(USER_BIN_PATH)/$(BINARY_NAME)"
	@echo ""
	@# 检查 PATH
	@if echo $$PATH | grep -q "$(USER_BIN_PATH)"; then \
		echo "✓ $(USER_BIN_PATH) 已在 PATH 中"; \
		echo "  你可以直接运行: $(BINARY_NAME)"; \
	else \
		echo "⚠️  警告: $(USER_BIN_PATH) 不在 PATH 中"; \
		echo "  请添加以下内容到你的 shell 配置文件:"; \
		echo ""; \
		if [ -n "$$ZSH_VERSION" ]; then \
			echo "  # 添加到 ~/.zshrc"; \
		elif [ -n "$$BASH_VERSION" ]; then \
			echo "  # 添加到 ~/.bashrc"; \
		else \
			echo "  # 添加到你的 shell 配置文件"; \
		fi; \
		echo "  export PATH=\"$(USER_BIN_PATH):\$$PATH\""; \
		echo ""; \
		echo "  然后运行: source ~/.zshrc (或 ~/.bashrc)"; \
		echo "  或者重新打开终端"; \
	fi

# 卸载
.PHONY: uninstall
uninstall:
	@echo "卸载 $(BINARY_NAME)..."
	@# 尝试从系统目录卸载
	@if [ -f $(INSTALL_PATH)/$(BINARY_NAME) ]; then \
		echo "从系统目录卸载..."; \
		sudo rm -f $(INSTALL_PATH)/$(BINARY_NAME); \
	fi
	@# 尝试从用户目录卸载
	@if [ -f $(USER_BIN_PATH)/$(BINARY_NAME) ]; then \
		echo "从用户目录卸载..."; \
		rm -f $(USER_BIN_PATH)/$(BINARY_NAME); \
	fi
	@echo "卸载完成"

# 清理编译产物
.PHONY: clean
clean:
	@echo "清理编译产物..."
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*
	@echo "清理完成"

# 格式化代码
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 检查代码
.PHONY: vet
vet:
	@echo "检查代码..."
	go vet ./...

# 测试
.PHONY: test
test:
	@echo "运行测试..."
	go test -v ./...

# 交叉编译
.PHONY: build-all
build-all:
	@echo "交叉编译..."
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 $(GO_FILES)
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 $(GO_FILES)
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_NAME)-darwin-amd64 $(GO_FILES)
	# macOS ARM64 (M1/M2)
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY_NAME)-darwin-arm64 $(GO_FILES)
	@echo "交叉编译完成"

# 创建发布包
.PHONY: release
release: clean build-all
	@echo "创建发布包..."
	mkdir -p release
	# 为每个平台创建 tar.gz 包
	tar czf release/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64 README.md config.toml.example
	tar czf release/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64 README.md config.toml.example
	tar czf release/$(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64 README.md config.toml.example
	tar czf release/$(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64 README.md config.toml.example
	# 清理临时文件
	rm -f $(BINARY_NAME)-*
	@echo "发布包创建完成: release/"

# 检查依赖
.PHONY: check-deps
check-deps:
	@echo "检查系统依赖..."
	@# 检查 Go
	@if command -v go >/dev/null 2>&1; then \
		echo "✓ Go 已安装: $$(go version)"; \
	else \
		echo "✗ Go 未安装"; \
		echo "  请访问 https://golang.org/dl/ 下载安装"; \
	fi
	@# 检查 Docker
	@if command -v docker >/dev/null 2>&1; then \
		echo "✓ Docker 已安装: $$(docker --version)"; \
	else \
		echo "✗ Docker 未安装"; \
		echo "  请访问 https://docs.docker.com/get-docker/ 安装"; \
	fi
	@# 检查 Restic
	@if command -v restic >/dev/null 2>&1; then \
		echo "✓ Restic 已安装: $$(restic version)"; \
	else \
		echo "✗ Restic 未安装"; \
		echo "  安装方法:"; \
		echo "  - macOS: brew install restic"; \
		echo "  - Ubuntu/Debian: sudo apt install restic"; \
		echo "  - 其他: https://restic.readthedocs.io/en/stable/020_installation.html"; \
	fi

# 显示帮助
.PHONY: help
help:
	@echo "Minecraft 备份工具 - Makefile 帮助"
	@echo ""
	@echo "可用命令:"
	@echo "  make build        - 编译程序"
	@echo "  make run          - 直接运行程序"
	@echo "  make install      - 安装到系统 (/usr/local/bin，需要 sudo)"
	@echo "  make install-user - 安装到用户目录 (~/.local/bin，不需要 sudo)"
	@echo "  make uninstall    - 卸载程序"
	@echo "  make clean        - 清理编译产物"
	@echo "  make fmt          - 格式化代码"
	@echo "  make vet          - 检查代码"
	@echo "  make test         - 运行测试"
	@echo "  make build-all    - 交叉编译所有平台"
	@echo "  make release      - 创建发布包"
	@echo "  make check-deps   - 检查系统依赖"
	@echo "  make help         - 显示此帮助信息"
	@echo ""
	@echo "快速开始:"
	@echo "  1. make check-deps  # 检查依赖"
	@echo "  2. make build       # 编译"
	@echo "  3. make install-user # 安装到用户目录（推荐）" 