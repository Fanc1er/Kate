# User Instruction Memory

This file records user instructions, preferences, and teachings for reference in future interactions.

## Format

[Project Knowledge Summary]
- Date: YYYY-MM-DD
- Context: Discovered by Agent while performing [specific task description]
- Category: [Operations & Deployment|Build Methods|Testing Methods|Troubleshooting & Debugging|Workflow & Collaboration|Environment Configuration]
- Instructions:
  - [knowledge points]

## Entries

[Project Knowledge Summary]
- Date: 2026-08-27
- Context: 修复全站 Tailwind 工具类失效问题时发现
- Category: Troubleshooting & Debugging
- Instructions:
  - 修改 postcss.config.js / tailwind.config.js 等构建管道配置后，运行中的 vite dev server（5173）不会自动加载新配置，必须重启 dev server 才生效；`npm run build` 每次全新进程不受影响
  - 验证 dev 模式样式管道是否生效：`curl http://localhost:5173/src/styles/tailwind.css`，响应中 @tailwind 指令原样保留=管道未生效，出现 `.gap-3 {` 等编译产物=已生效
  - 项目模板大量使用 Tailwind 工具类（preflight 已关闭以兼容 @opentiny/vue-theme）；border 类依赖入口 tailwind.css 中手动补的 box-sizing/border-style 基础规则，勿删

[Project Knowledge Summary]
- Date: 2026-08-27
- Context: 端到端冒烟测试搭建时发现
- Category: Operations & Deployment
- Instructions:
  - 本地联调账号：admin / Admin@123456（后端 dev server 由 CINSIGHT_SUPER_ADMIN_PASS 环境变量注入，DB 在 /tmp/opencode/kate-it/data/cinsight.db）
  - 查库用 python3 内置 sqlite3 模块（环境无 sqlite3 CLI）：python3 -c "import sqlite3; ..."
  - worker 独立进程启动：cd backend && CINSIGHT_MASTER_URL=http://localhost:8080 CINSIGHT_WORKER_BOOT_TOKEN=<token> go run ./cmd/worker；token 通过 admin JWT 调 POST /api/v1/worker/nodes 签发，响应 data.bootstrap_token
  - 任务分两种 scope：availability_probe（轻量探针，只写时序点）与 root（全量扫描，findings 落在 root 任务上），查 findings 时按 asset_id 过滤更直接
  - worker 节点注册成功后 master 即清除 boot_token_hash，重启 worker 必须先重新签发新 token，旧 token 不可复用
  - `go run` 前端/后端/worker 三个 dev 进程都不热加载业务代码：提交引擎/服务层修复后必须 kill 并重启对应进程，否则修复在线上不生效（已踩坑：CVE 去重、sparkline 结构修复一度未生效）
