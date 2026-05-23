# KiroX CLI

## 项目概述

AWS Builder ID (Kiro) 批量自动注册工具，包含 CLI 和 Web UI 两种使用方式。

## 技术栈

- **核心逻辑**: Go 1.24, bogdanfinn/tls-client (TLS 指纹模拟)
- **Web 后端**: Gin, gorilla/websocket
- **Web 前端**: Vue 3 + Vite + Element Plus + Pinia
- **部署**: Docker 多阶段构建

## 项目结构

```
├── main.go                 # CLI 入口 (原有)
├── cmd/server/main.go      # Web 服务入口
├── internal/               # 核心注册逻辑 (不要修改)
│   ├── core/               # 注册流程编排
│   ├── browser/            # 浏览器指纹模拟
│   ├── email/              # 邮箱服务 (Outlook IMAP / MoeMail)
│   ├── crypto/             # JWE / XXTEA 加密
│   └── http/               # TLS 客户端工具
├── server/                 # Web API 层
│   ├── server.go           # Gin 路由
│   ├── auth.go             # JWT 认证
│   ├── task_manager.go     # 任务调度
│   ├── handler_*.go        # API handlers
│   ├── ws_log.go           # WebSocket 日志
│   ├── embed.go            # 前端静态文件嵌入
│   └── dist/               # 前端构建产物 (git tracked)
├── web/                    # Vue 3 前端源码
├── Dockerfile              # 多阶段构建
└── docker-compose.yml      部署配置
```

## 开发命令

```bash
# CLI 模式 (需要本地 Go 环境)
go run main.go -n 5 -j 2 -p socks5://127.0.0.1:1080

# Web 模式 - 前端开发
cd web && npm run dev

# Web 模式 - 前端构建
cd web && npm run build

# Web 模式 - 后端编译
go build -o kirox-server ./cmd/server

# Docker 部署
docker compose up -d --build
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| ADMIN_PASSWORD | admin | Web UI 登录密码 |
| JWT_SECRET | kirox-default-secret-change-me | JWT 签名密钥 |
| MOEMAIL_BASE_URL | https://api.moemail.app | MoeMail API 地址 |
| MOEMAIL_API_KEY | (空) | MoeMail API Key |
| DATA_DIR | ./data | 数据持久化目录 |
| PORT | 8080 | Web 服务端口 |

## 注意事`server/dist/` 是前端构建产物，修改前端后需要重新 `npm run build`
- 前端构建输出到 `server/dist/`，通过 Go embed 嵌入二进制
- Docker 构建时会自动执行 `go mod tidy` 拉取依赖
