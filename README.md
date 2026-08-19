# Kate

全栈应用：Vue 3 + Vite + TypeScript 前端，Go 后端。

## 结构

- `frontend/` — Vue 3 + Vite 5 + TypeScript，开发端口 5173，`/api` 反向代理到后端
- `backend/` — Go 1.25 标准库 `net/http` 服务，端口 8080（`PORT` 环境变量可覆盖）

## 快速开始

```bash
# 后端
cd backend && go run .

# 前端
cd frontend && npm install && npm run dev
```

浏览器访问 `http://localhost:5173`，前端会通过 `/api` 代理调用后端。

## 验证

```bash
# 后端
cd backend && go vet ./... && go build .

# 前端
cd frontend && npm run build   # vue-tsc 类型检查 + vite 构建
```

## 接口

- `GET /api/health` — 健康检查，返回服务名、状态、版本
