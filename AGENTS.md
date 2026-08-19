# AGENTS.md

Kate 是一个全栈应用：`frontend/` 为 Vue 3 + Vite 5 + TypeScript，`backend/` 为 Go 1.25（标准库 `net/http`）。

## 开发命令

```bash
# 后端（端口 8080，可用 PORT 环境变量覆盖）
cd backend && go run .

# 前端开发服务器（端口 5173）
cd frontend && npm install && npm run dev
```

## 验证命令

```bash
# 后端：vet + build
cd backend && go vet ./... && go build .

# 前端：类型检查 + 生产构建（vue-tsc -b && vite build）
cd frontend && npm run build

# 前端单独类型检查
cd frontend && npm run typecheck
```

改代码后按 `后端 go vet/build → 前端 typecheck/build` 顺序验证。项目没有 ESLint/Prettier 配置，无需运行 lint。

## 架构要点

- Vite `server.proxy` 将 `/api` 转发到 `http://localhost:8080`，后端只实现 `/api/*` 路由（当前仅 `GET /api/health`）。
- 前端通过相对路径 `/api/...` 调后端，不要写死后端地址，也不要做 CORS 配置。
- Vite `allowedHosts` 已含 `.monkeycode-ai.online`，供预览域名使用，勿删。
- 后端无第三方依赖，保持标准库 `net/http`；新增路由用 `http.NewServeMux`。
- 联调验证：后端跑在 8080，前端跑在 5173，`curl localhost:5173/api/health` 应返回后端 JSON。

## 注意

- `tsconfig.node.json` 是 composite 引用项目，**不要**给它设置 `noEmit`（会导致 TS6310 报错）。
- Node 18+ 是 Vite 5 的最低要求。
