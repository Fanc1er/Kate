# CInsight 智能监测平台 — 实施任务列表

Feature Name: cinsight-platform
Updated: 2026-08-19
关联文档: [requirements.md](requirements.md) / [design.md](design.md)

---

## 阶段 1 — MVP 闭环（RBAC + 资产 + 任务 + 证据链）

### 1.1 技术底座
- [ ] 初始化 Go 模块与 Gin 脚手架（cmd/master、cmd/worker）
- [ ] 引入依赖：Gin、GORM、SQLite 驱动（WAL 模式）、BadgerDB、Bleve、ants、gobreaker、fsnotify、swaggo/swag、gorilla/websocket
- [ ] 统一响应中间件（code/message/data 格式）与错误码定义（见 design 错误码枚举）
- [ ] 统一约定落地：分页/排序/筛选参数、RFC3339 时间、鉴权分层（JWT / API Token / Worker Bootstrap）
- [ ] 目录骨架：internal/master/{controller,service,repository,middleware,routes}、internal/worker/{engine,scheduler,reporter}、pkg/{db,badger,bleve,storage,utils}
- [ ] 配置管理落地（环境变量清单：PORT/DB_PATH/DATA_DIR/JWT_SECRET/RULES_DIR 等，见 design 配置表）
- [ ] Swagger 文档集成（swag init 初始化 + /swagger/* 端点暴露，阶段 3 全量注解）【必执行】
- [ ] 全量业务表结构迁移（assets/findings/events/tickets/evidence/audit_logs/api_tokens/notify_channels/noise_rules/worker_nodes/scan_plans/availability_points/trend_points）

### 1.2 认证与 RBAC
- [ ] organizations/users/user_orgs 表迁移
- [ ] 登录接口（bcrypt 校验 + JWT 签发，不含 org_id）
- [ ] 组织选择接口 POST /api/v1/auth/select-org（换取带 org_id 的 JWT）
- [ ] JWT 中间件 + X-Org-Id 校验 + RBAC 中间件（RequireRoles/RequireWrite）
- [ ] super_admin 全局 org_id=0 平台查询通道
- [ ] super_admin 禁止加入 user_orgs 约束（触发器/服务层校验）
- [ ] Repository 层 org_id 强制过滤守卫（缺省 org_id 拒绝查询）
- [ ] 登录锁定阈值控制

### 1.3 前端基础框架
- [ ] 引入 TinyVue (@opentiny/vue) + Pinia + Vue Router 4 + ReconnectingWebSocket + ECharts
- [ ] 前端目录结构落地（src/api|stores|router|layouts|views|components|utils，见 design 前端架构）
- [ ] axios 封装 + 模块化 API 层（Bearer + X-Org-Id + 401 跳转 + 模块拆分）
- [ ] Pinia store 划分（auth/menu/asset/event/dashboard）
- [ ] 登录页 /login + 组织选择卡片页
- [ ] 基于 role 的动态 addRoute() 路由与菜单（含 super_admin 平台管理/选择组织双入口）
- [ ] 顶部导航（组织名 + 角色 Tag + 切换组织/退出）

### 1.4 资产模块
- [ ] 资产 CRUD（表单：URL 归一化/名称/分组/重要程度/备注）
- [ ] URL 标准化与 BadgerDB MD5 去重
- [ ] 资产列表（虚拟滚动/模糊搜索/筛选）
- [ ] 资产画像抽屉（技术栈指纹/ICP/子域名/SSL 倒计时/端口快照）
- [ ] 资产变更追踪（标题/技术栈/状态码/端口变动历史）
- [ ] 微信公众号资产字段（公众号名/微信号/头像/粉丝数/简介/认证状态/文章数，扩展自 R5.2）

### 1.5 任务调度与可用性引擎
- [ ] 任务表与策略模板表迁移
- [ ] SQLite 连接与 WAL 配置（journal_mode/busy_timeout/synchronous/foreign_keys，单写连接池）
- [ ] GORM 迁移策略落地（AutoMigrate + schema_migrations + 种子数据）
- [ ] 核心表索引落地（org_id/状态/时间索引，见 design 索引设计）
- [ ] BadgerDB-SQLite 缓存一致性（写入先 Badger 后异步 SQLite + 每小时对账）
- [ ] Master 任务创建/下发/拉取接口（pending→processing→completed）
- [ ] Worker 调度器（拉取 + 执行 + 回传）
- [ ] 可用性监测引擎（HTTP 探针 + 连续 3 次失败宕机判定）
- [ ] Worker Outbox 本地缓存与断网回传
- [ ] Master 启动超时任务对账（30min 重置 processing→pending）
- [ ] 断点续扫（local:crawled:{task_id}）

### 1.6 证据链
- [ ] 证据 gzip 落盘 /data/evidence/{date}/ + MD5 去重 + SHA-256 入库
- [ ] 证据读取时 Hash 强制校验（不一致返回 EVIDENCE_TAMPERED）
- [ ] Worker 侧证据生成（Req/Resp/HTML 快照/代码定位行号）+ 结果回传链路
- [ ] Worker 页面渲染截图采集（截图取证）
- [ ] 截图上传接口（POST /api/v1/evidence/screenshots，Base64 或文件流，鉴权 + SHA-256 校验）
- [ ] 前端全屏证据抽屉（Req/Resp 分屏 + HTML 行号高亮 + 截图标签页 + 下载按钮）

### 1.7 仪表盘与报告
- [ ] 统计卡片 + 7 天趋势 + 风险 Top10
- [ ] 引擎覆盖率雷达图（10 大引擎检测覆盖率）
- [ ] WebSocket 实时事件滚动 + 指数退避重连（/api/v1/ws/events 订阅协议）
- [ ] ECharts 图表集成（雷达图/趋势图/折线图/可用性点阵图）
- [ ] 结构化日志 + request_id 请求追踪中间件
- [ ] 报告导出（PDF 含水印 / Excel 漏洞清单）

### 1.8 阶段 1 单元测试【必执行】
- [ ] RBAC 权限矩阵单元测试（四角色 × 读写操作，表驱动）
- [ ] 认证服务单元测试（bcrypt 校验/JWT 签发/组织选择/登录锁定）
- [ ] 证据服务单元测试（gzip 落盘/SHA-256 校验/MD5 去重/篡改检测）
- [ ] 任务调度单元测试（状态机流转/超时对账/断点续扫）

### 阶段 1 验收
- [ ] go vet ./... && go build . 通过【必执行】
- [ ] go test ./... 单元测试全部通过（RBAC/认证/证据/调度）【必执行】
- [ ] 前端 vue-tsc + vite build 通过【必执行】
- [ ] 联调：登录→建资产→下发任务→Worker 执行→证据展示全链路可跑【必执行】

---

## 阶段 2 — 引擎扩展（10 大引擎全量）

### 2.1 统一引擎注册
- [ ] EngineRegistry 与 Engine 接口（Name/Enabled/Run）
- [ ] 策略模板引擎开关 + 并发/超时/速率限制下发

### 2.2 引擎实现
- [ ] 漏洞扫描引擎（POC + Fuzzing + 参数注入，context 30s 超时 + ants 并发）
- [ ] 内容安全引擎（AI 文本分类 + 敏感词正则双判定 + 敏感信息识别 + 篡改基线）
- [ ] 暗链挂马引擎（隐藏外链/木马特征/双 UA 对比）
- [ ] Webshell 检测引擎（路径枚举 + 特征码 + 流量特征）
- [ ] 钓鱼检测引擎（模板比对 + Levenshtein + 证书异常）
- [ ] 端口服务监测引擎（TCP SYN 扫描 + Banner 指纹 + 高危暴露告警）
- [ ] 可用性监测引擎扩展（DNS 监控：解析 IP 变更/解析失败/劫持检测）
- [ ] 可用性监测引擎扩展（PING 监控：丢包率/延迟/ICMP 不可达告警）
- [ ] DNS 安全引擎（多节点解析对比 + 污染检测 + 子域名爆破）
- [ ] 信誉监测引擎（IP/域名威胁情报查询）
- [ ] 安全情报引擎（CVE/CNVD/CNNVD 订阅 + 资产影响匹配）

### 2.3 事件中心与漏洞管理
- [ ] 事件列表 + 状态流转（待处理→处理中→已关闭→已归档）
- [ ] 事件类型筛选（12 类）+ 降噪规则配置（白名单/忽略/聚合/风暴抑制）
- [ ] 漏洞列表（等级/状态/引擎筛选）+ 证据链抽屉接入
- [ ] 工单闭环（确认→派发→修复→复测→归档）+ SOP 挂载
- [ ] 告警风暴抑制（单资产每小时 5 条上限）

### 2.4 引擎相关前端模块
- [ ] 内容安全监测页（敏感内容/信息泄漏/篡改对比 + 截图缩略图）
- [ ] 暗链木马页（暗链列表/木马列表/双 UA 对比）
- [ ] Webshell 与钓鱼页
- [ ] 可用性网络页（12h 点阵图 + 24h 时序折线 + DNS/端口记录）
- [ ] 安全情报中心页（情报列表 + 受影响资产数 + 订阅配置）
- [ ] 任务队列监控页（排队/处理中/完成 + Worker 分配 + 断点续扫状态）

### 2.5 重组件集成
- [ ] Bleve 索引 + BatchIndexer（5s/50 条批量提交）
- [ ] BadgerDB 元数据缓存层（API 毫秒级响应）
- [ ] 异步批量持久化 SQLite/Bleve 通道

### 2.6 阶段 2 单元测试【必执行】
- [ ] 引擎契约单元测试（10 引擎 mock 输入 → finding 输出）
- [ ] 脱敏单元测试（身份证/手机号/邮箱/AccessKey 三时机脱敏）

### 阶段 2 验收
- [ ] 10 大引擎全部可开关执行【必执行】
- [ ] 事件/漏洞/工单全流程闭环【必执行】
- [ ] 万级列表虚拟滚动性能达标【必执行】
- [ ] go test ./... 全部通过 + 前端 vue-tsc + vite build 通过【必执行】

---

## 阶段 3 — 全量功能 + 平台化

### 3.1 团队与系统设置
- [ ] 成员管理（邀请/移除/禁用/改角色）— 仅 org_admin
- [ ] Worker 节点管理（心跳/负载/版本/Bootstrap Token）
- [ ] 通知渠道配置（钉钉/企微/飞书 Webhook + SMTP + 测试发送）
- [ ] 规则库管理（POC/敏感词/木马特征库 + 版本号）
- [ ] 审计日志（禁止修改删除）
- [ ] API Token 管理（细粒度权限/有效期）

### 3.2 平台管理
- [ ] 组织列表/创建/编辑/禁用 — 仅 super_admin
- [ ] 平台统计（总组织/总资产/总扫描/总事件）
- [ ] 平台 Worker 总览

### 3.3 集成与部署
- [ ] swaggo Swagger 文档全量注解
- [ ] API Token 认证中间件（独立于 JWT，scopes 细粒度 + 有效期）
- [ ] Webhook 事件推送（HMAC-SHA256 签名 + 重试 3 次 + 落库）
- [ ] 规则热更新（fsnotify）+ Worker 规则 Hash 同步
- [ ] 扫描授权拦截（白名单校验 + 内网 IP 禁止）
- [ ] gobreaker 目标熔断（连续失败 5 次）+ 授权违规熔断 Worker
- [ ] 三时机脱敏（入库/API 返回/报告生成）
- [ ] 反封禁（Proxy 配置 + 低速隐蔽模式）
- [ ] VictoriaMetrics 迁移接口预留（MetricsExporter，当前 SQLite 时序表）

### 任务 15 — Litestream 热备与部署【必执行】
- [ ] Litestream SQLite 实时流式热备（配置 + 复制验证）
- [ ] Docker 编排（Master + Worker + SQLite 卷 + 数据目录挂载）
- [ ] K8s 编排（Master 水平扩展读写分离 + Worker 弹性伸缩 HPA）
- [ ] 私有化单二进制一键安装打包（零外部依赖）
- [ ] 部署验证：Docker 起服务 → 探活 /api/health → 建资产 → 下发任务全链路通过

### 3.4 报告中心全量
- [ ] 报告模板（执行摘要/漏洞详情/内容安全/可用性统计/整改建议）
- [ ] Cron 定时计划（绑定资产分组 + 策略模板 + 时间窗口）
- [ ] 定时报告（Cron 生成周报/月报）
- [ ] 报告截图合集导出（按资产/时间范围）

### 阶段 3 验收
- [ ] 全部 15 个功能模块上线【必执行】
- [ ] 平台管理/团队管理权限隔离正确【必执行】
- [ ] 集成测试：Master + 单 Worker 全链路（任务下发→引擎执行→结果回传→证据入库→前端展示）【必执行】
- [ ] 容灾演练：Worker 断网恢复（Outbox 回传）、Master 重启对账、熔断触发【必执行】
- [ ] 部署验证：Docker/K8s/单二进制三种方式启动 + 探活 + 全链路通过【必执行】
- [ ] go test ./... 全部通过 + 前端 vue-tsc + vite build 通过【必执行】
