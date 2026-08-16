# ─────────────────────────────────────────────────────────
#  Fluxa — 项目 Makefile
#  用法: make help
# ─────────────────────────────────────────────────────────

# 项目元信息
APP_NAME   := fluxa
GO_MODULE  := github.com/amigoer/fluxa
MAIN_PKG   := ./cmd/server
BIN_DIR    := ./bin
BINARY     := $(BIN_DIR)/$(APP_NAME)

# 数据库连接（与 docker-compose 保持一致）
DATABASE_URL ?= postgres://fluxa:fluxa@localhost:5432/fluxa?sslmode=disable

# 导出环境变量给 Go 进程
export FLUXA_DATABASE_URL      := $(DATABASE_URL)
export FLUXA_LISTEN_ADDR       ?= :8080
export FLUXA_SESSION_COOKIE_SECURE ?= false
export FLUXA_BASE_URL          ?= http://localhost:8080

# 前端目录
FRONTEND_DIR := ./frontend

.PHONY: help
help: ## 显示所有可用命令
	@echo ""
	@echo "  Fluxa 开发命令"
	@echo "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""

# ─────────────────── 基础设施 ───────────────────

.PHONY: infra-up
infra-up: ## 启动 PostgreSQL（docker compose）
	docker compose up -d
	@echo "⏳ 等待 PostgreSQL 就绪..."
	@until docker compose exec postgres pg_isready -U fluxa > /dev/null 2>&1; do sleep 1; done
	@echo "✅ PostgreSQL 已就绪"

.PHONY: infra-down
infra-down: ## 停止基础设施容器
	docker compose down

.PHONY: infra-reset
infra-reset: ## 销毁数据卷并重建（⚠️ 清除所有数据）
	docker compose down -v
	$(MAKE) infra-up

# ─────────────────── 数据库 ───────────────────

.PHONY: db-status
db-status: ## 查看数据库连接状态
	@docker compose exec postgres pg_isready -U fluxa

.PHONY: db-psql
db-psql: ## 打开 psql 交互终端
	docker compose exec postgres psql -U fluxa -d fluxa

# ─────────────────── 后端 ───────────────────

.PHONY: build
build: ## 编译 Go 服务端二进制
	@mkdir -p $(BIN_DIR)
	go build -o $(BINARY) $(MAIN_PKG)
	@echo "✅ 已编译 → $(BINARY)"

.PHONY: run
run: ## 运行后端服务（go run，支持快速迭代）
	go run $(MAIN_PKG)

.PHONY: test
test: ## 运行全部 Go 测试
	go test ./... -v -count=1

.PHONY: lint
lint: ## 运行 Go 静态检查（需安装 golangci-lint）
	golangci-lint run ./...

.PHONY: tidy
tidy: ## 整理 Go 依赖
	go mod tidy

# ─────────────────── 前端 ───────────────────

.PHONY: fe-install
fe-install: ## 安装前端依赖
	cd $(FRONTEND_DIR) && npm install

.PHONY: fe-dev
fe-dev: ## 启动前端开发服务器（Vite，含 API 代理）
	cd $(FRONTEND_DIR) && npm run dev

.PHONY: fe-build
fe-build: ## 构建前端生产产物（输出到 web/dist）
	cd $(FRONTEND_DIR) && npm run build

.PHONY: fe-lint
fe-lint: ## 运行前端 lint（oxlint）
	cd $(FRONTEND_DIR) && npm run lint

# ─────────────────── 一键启动 ───────────────────

.PHONY: dev
dev: infra-up ## 🚀 一键启动：基础设施 + 后端（前端请另开终端 make fe-dev）
	@echo ""
	@echo "🚀 基础设施已就绪，正在启动后端..."
	@echo "💡 前端开发服务器请另开终端运行: make fe-dev"
	@echo ""
	go run $(MAIN_PKG)

.PHONY: dev-full
dev-full: infra-up ## 🚀 一键启动全栈（后端 + 前端并行，Ctrl+C 全部停止）
	@echo ""
	@echo "🚀 启动全栈开发环境..."
	@echo ""
	@trap 'kill 0' EXIT; \
		(cd $(FRONTEND_DIR) && npm run dev) & \
		sleep 2 && go run $(MAIN_PKG) & \
		wait

# ─────────────────── 清理 ───────────────────

.PHONY: clean
clean: ## 清理编译产物
	rm -rf $(BIN_DIR)
	rm -rf web/dist
	@echo "🧹 已清理"

.DEFAULT_GOAL := help
