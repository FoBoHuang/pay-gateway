# Makefile for Pay Gateway

# 变量定义
APP_NAME=pay-gateway
VERSION=1.0.0
BUILD_TIME=$(shell date +%Y-%m-%d_%H:%M:%S)
GIT_COMMIT=$(shell git rev-parse --short HEAD)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Go相关变量
GO=go
GOFMT=gofmt
GOLINT=golangci-lint
GOTEST=go test
GOBUILD=go build

# 目录
CMD_DIR=./cmd/server
BUILD_DIR=./build
DIST_DIR=./dist

# 默认目标
.PHONY: all
all: clean fmt lint test build

# 清理
.PHONY: clean
clean:
	@echo "清理构建文件..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@$(GO) clean

# 格式化代码
.PHONY: fmt
fmt:
	@echo "格式化代码..."
	@$(GOFMT) -s -w .
	@$(GO) mod tidy

# 代码检查
.PHONY: lint
lint:
	@echo "运行代码检查..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run; \
	else \
		echo "golangci-lint not found, installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.54.2; \
		$(GOLINT) run; \
	fi

# 运行测试
.PHONY: test
test:
	@echo "运行测试..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./...

# 测试覆盖率
.PHONY: test-coverage
test-coverage: test
	@echo "生成测试覆盖率报告..."
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"

# 构建应用
.PHONY: build
build:
	@echo "构建应用..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)

# 构建Linux版本
.PHONY: build-linux
build-linux:
	@echo "构建Linux版本..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 $(CMD_DIR)

# 构建所有平台版本
.PHONY: build-all
build-all:
	@echo "构建所有平台版本..."
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 $(CMD_DIR)
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 $(CMD_DIR)
	@GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe $(CMD_DIR)

# 运行应用
.PHONY: run
run:
	@echo "运行应用..."
	@$(GO) run $(CMD_DIR)

# 开发模式运行
.PHONY: dev
dev:
	@echo "开发模式运行..."
	@SERVER_MODE=debug $(GO) run $(CMD_DIR)

# 安装依赖
.PHONY: deps
deps:
	@echo "安装依赖..."
	@$(GO) mod download
	@$(GO) mod verify

# 更新依赖
.PHONY: deps-update
deps-update:
	@echo "更新依赖..."
	@$(GO) get -u ./...
	@$(GO) mod tidy

# Docker相关
.PHONY: docker-build
docker-build:
	@echo "构建Docker镜像..."
	@docker build -t $(APP_NAME):$(VERSION) .
	@docker build -t $(APP_NAME):latest .

.PHONY: docker-run
docker-run:
	@echo "运行Docker容器..."
	@docker run -d --name $(APP_NAME) -p 8080:8080 $(APP_NAME):latest

.PHONY: docker-stop
docker-stop:
	@echo "停止Docker容器..."
	@docker stop $(APP_NAME) || true
	@docker rm $(APP_NAME) || true

# Docker Compose相关
.PHONY: compose-up
compose-up:
	@echo "启动Docker Compose服务..."
	@docker-compose up -d

.PHONY: compose-down
compose-down:
	@echo "停止Docker Compose服务..."
	@docker-compose down

.PHONY: compose-logs
compose-logs:
	@echo "查看Docker Compose日志..."
	@docker-compose logs -f

.PHONY: compose-restart
compose-restart:
	@echo "重启Docker Compose服务..."
	@docker-compose restart

# 数据库相关
.PHONY: db-migrate
db-migrate:
	@echo "运行数据库迁移..."
	@$(GO) run $(CMD_DIR) migrate

.PHONY: db-seed
db-seed:
	@echo "填充数据库种子数据..."
	@$(GO) run $(CMD_DIR) seed

# 健康检查
.PHONY: health
health:
	@echo "检查服务健康状态..."
	@curl -f http://localhost:8080/health || echo "服务未运行"

# 生成API文档
.PHONY: docs
docs:
	@echo "生成API文档..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g $(CMD_DIR)/main.go -o ./docs; \
	else \
		echo "swag not found, installing..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
		swag init -g $(CMD_DIR)/main.go -o ./docs; \
	fi

# 性能测试
.PHONY: bench
bench:
	@echo "运行性能测试..."
	@$(GOTEST) -bench=. -benchmem ./...

# 安全扫描
.PHONY: security
security:
	@echo "运行安全扫描..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found, installing..."; \
		go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; \
		gosec ./...; \
	fi

# 帮助信息
.PHONY: help
help:
	@echo "可用的命令:"
	@echo "  all          - 清理、格式化、检查、测试并构建"
	@echo "  clean        - 清理构建文件"
	@echo "  fmt          - 格式化代码"
	@echo "  lint         - 运行代码检查"
	@echo "  test         - 运行测试"
	@echo "  test-coverage- 生成测试覆盖率报告"
	@echo "  build        - 构建应用"
	@echo "  build-linux  - 构建Linux版本"
	@echo "  build-all    - 构建所有平台版本"
	@echo "  run          - 运行应用"
	@echo "  dev          - 开发模式运行"
	@echo "  deps         - 安装依赖"
	@echo "  deps-update  - 更新依赖"
	@echo "  docker-build - 构建Docker镜像"
	@echo "  docker-run   - 运行Docker容器"
	@echo "  docker-stop  - 停止Docker容器"
	@echo "  compose-up   - 启动Docker Compose服务"
	@echo "  compose-down - 停止Docker Compose服务"
	@echo "  compose-logs - 查看Docker Compose日志"
	@echo "  compose-restart- 重启Docker Compose服务"
	@echo "  db-migrate   - 运行数据库迁移"
	@echo "  db-seed      - 填充数据库种子数据"
	@echo "  health       - 检查服务健康状态"
	@echo "  docs         - 生成API文档"
	@echo "  bench        - 运行性能测试"
	@echo "  security     - 运行安全扫描"
	@echo "  help         - 显示帮助信息"
