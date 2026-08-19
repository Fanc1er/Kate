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
- [ ] 统一约定落地：响应格式 {code/message/data}（code=0 成功，业务码非 0）+ 分页 page/page_size（默认 20 上限 200，返回 list+total）/排序 sort/filter 筛选/RFC3339 时间/鉴权分层（JWT / API Token / Worker Bootstrap）
- [ ] 目录骨架：internal/master/{controller,service,repository,middleware,routes}、internal/worker/{engine,scheduler,reporter}、pkg/{db,badger,bleve,storage,utils}
- [ ] 配置管理落地（环境变量清单：PORT/DB_PATH/DATA_DIR/JWT_SECRET/RULES_DIR 等，见 design 配置表）
- [ ] Swagger 文档集成（swag init 初始化 + /swagger/* 端点暴露，CINSIGHT_SWAGGER_ENABLED 开关生产默认关闭，阶段 3 全量注解）【必执行】
- [ ] 全量业务表结构迁移（organizations/users/user_orgs/assets/vulnerabilities/alerts/findings/scan_tasks/events/tickets/evidence/evidence_files/audit_logs/api_tokens/notify_channels/notify_routes/noise_rules/scan_whitelists/worker_nodes/scan_policies/scan_plans/intel_subscriptions/report_templates/reports/webhooks/wechat_assets/availability_points/trend_points/sensitive_info_hits/rule_definitions）

### 1.2 认证与 RBAC
- [ ] organizations/users/user_orgs 表迁移
- [ ] 登录接口（bcrypt 校验 + JWT 签发，不含 org_id）
- [ ] 当前用户信息接口（GET /api/v1/auth/me：用户名/邮箱/角色/当前组织/头像/权限码集，仅依赖 JWT 不需 X-Org-Id）+ 登出接口（POST /api/v1/auth/logout，refresh token 入 jti 黑名单）
- [ ] 禁用用户/禁用组织登录拦截（users.status / user_orgs.status / org.status 校验，403 USER_DISABLED/ORG_DISABLED + 后续 API 校验失效）
- [ ] refresh token 机制（POST /api/v1/auth/refresh，access 15min + refresh 7d，jti 黑名单）
- [ ] 登出/换组织/改密/重置后 token 失效（黑名单生效）
- [ ] 组织选择接口 POST /api/v1/auth/select-org（换取带 org_id 的 JWT）
- [ ] 切换组织前端处理（刷新数据 + 关闭旧 WS 重建连接绑定新 org_id）
- [ ] JWT 中间件 + X-Org-Id 校验 + RBAC 中间件（RequireRoles/RequireWrite）
- [ ] super_admin 全局 org_id=0 平台查询通道
- [ ] super_admin 禁止加入 user_orgs 约束（触发器/服务层校验）
- [ ] Repository 层 org_id 强制过滤守卫（缺省 org_id 拒绝查询）
- [ ] 登录锁定阈值控制
- [ ] 密码重置流程（POST /api/v1/auth/forgot-password + reset-password，重置后失效旧 token）
- [ ] 系统级邮件发送（CINSIGHT_SMTP_*：验证码/邀请邮件，验证码 5min 一次性，独立于组织通知渠道）
- [ ] 登录态改密（POST /api/v1/auth/change-password：校验旧密码 + 符合密码策略 + 改后失效全部 refresh token）
- [ ] 首启引导初始化 super_admin/默认策略/降噪规则（--init-super-admin 或首启向导，未初始化禁用平台管理）

### 1.3 前端基础框架
- [ ] 引入 TinyVue (@opentiny/vue) + Pinia + Vue Router 4 + ReconnectingWebSocket + ECharts + DOMPurify
- [ ] 前端目录结构落地（src/api|stores|router|layouts|views|components|utils，见 design 前端架构）
- [ ] axios 封装 + 模块化 API 层（Bearer + X-Org-Id + 401 跳转 + 模块拆分：http/auth/asset/task/event/finding/report/dashboard/intel/admin/ws）
- [ ] Pinia store 划分（auth/menu/asset/event/dashboard）
- [ ] 登录页 /login + 组织选择卡片页 + 忘记密码/重置密码页（forgot-password + reset-password 表单 + 验证码 5min 提示）
- [ ] 基于 role 的动态 addRoute() 路由与菜单（含 super_admin 平台管理/选择组织双入口）
- [ ] 按钮级权限指令 v-permission + 权限码表（src/config/permissions.ts，按 design「RBAC 权限矩阵与权限码」清单落地），菜单/路由/按钮三级一致
- [ ] 顶部导航（组织名 + 角色 Tag + 切换组织/退出）
- [ ] 全局错误边界 + 骨架屏 + 请求失败 Toast（axios 拦截器 + 异常兜底页）
- [ ] WebSocket 断线提示条 + 重连自动恢复清除
- [ ] 乐观锁 409 冲突前端提示（"数据已被他人修改，请刷新后重试"）

### 1.4 资产模块
- [ ] 资产后端 API：CRUD（GET/POST /api/v1/assets，GET/PUT/DELETE /api/v1/assets/:id）
- [ ] 资产后端 API：画像（GET /api/v1/assets/:id/profile）
- [ ] 资产后端 API：变更追踪（GET /api/v1/assets/:id/history）
- [ ] 资产后端 API：URL 归一化 + BadgerDB MD5 防重
- [ ] 资产后端 API：微信公众号资产完整 CRUD（GET/POST /api/v1/wechat-assets，PUT/DELETE /api/v1/wechat-assets/:id）
- [ ] 资产批量操作 API：batch-scan（批量加入扫描）/ batch-delete / batch-group（批量改分组）/ batch-import（URL/CSV 批量导入 + 模板下载 + 逐行校验报告）
- [ ] 资产分组为字符串标签（group_name 自由填写 + 列表分组筛选下拉 distinct 计数 + 定时计划按组名精确匹配）
- [ ] 资产导入模板下载（GET /api/v1/assets/import-template）+ 当前筛选结果 CSV 导出（GET /api/v1/assets/export）
- [ ] 资产列表前端（虚拟滚动/模糊搜索/筛选 + 多选批量操作栏 + 空状态引导）
- [ ] 资产画像抽屉前端（技术栈指纹/ICP/子域名/SSL 倒计时/端口快照）
- [ ] 资产变更追踪前端（标题/技术栈/状态码/端口变动历史）
- [ ] 微信公众号资产前端字段（公众号名/微信号/头像/粉丝数/简介/认证状态/文章数，扩展自 R5.2）
- [ ] 资产行内"立即扫描"快捷操作 + URL 一键复制

### 1.5 任务调度与可用性引擎
- [ ] 任务表与策略模板表迁移
- [ ] SQLite 连接与 WAL 配置（journal_mode/busy_timeout/synchronous/foreign_keys，单写连接池 + 连接参数 MaxOpenConns(1)/ConnMaxLifetime(30m)/ConnMaxIdleTime(10m)）
- [ ] GORM 迁移策略落地（AutoMigrate + schema_migrations + 种子数据）
- [ ] 核心表索引落地（org_id/状态/时间索引，见 design 索引设计）
- [ ] BadgerDB-SQLite 缓存一致性（写入先 Badger 后异步 SQLite + 每小时对账）
- [ ] Master 任务创建/下发/拉取接口（pending→processing→completed）
- [ ] 任务分发 Pull 模型（Worker 轮询拉取 + 原子置 processing 防双拉 + 按心跳 load 最低优先分配 + 任务不拆分）
- [ ] 任务去重（同 org+asset+policy 存在 pending/processing 时返回 3001 TASK_STATE_CONFLICT）
- [ ] 任务详情接口（GET /api/v1/tasks/:id，状态/进度/执行日志/结果统计/Worker 分配）
- [ ] 任务队列监控与断点续扫状态（GET /api/v1/tasks/queue 排队/处理中/已完成 + Worker 分配 + GET /api/v1/tasks/:id/progress）
- [ ] 任务停止/删除/批量停止/失败重跑（POST :id/stop 置 cancelled 标记 stopped_by_user + Worker stop_check 感知中止回传 cancelled、DELETE :id、POST batch-stop、POST :id/rerun、POST batch-rerun）
- [ ] Worker 调度器（拉取 + 执行 + 回传）
- [ ] Worker 心跳上报（POST /api/v1/worker/heartbeat，节点心跳/负载/版本更新）
- [ ] Worker 注册握手（POST /api/v1/worker/register：Bootstrap Token 一次性换长期凭证 client_id+client_secret，后续心跳/拉取/回传用长期凭证，支持吊销/重发）
- [ ] 可用性监测引擎（HTTP 探针 + 连续 3 次失败宕机判定）
- [ ] 可用性引擎接入 MultiUAAssessor（端间状态码/延迟不一致 + 端差异化宕机标记）
- [ ] Worker Outbox 本地缓存与断网回传
- [ ] 结果回传幂等键去重（result_id 唯一索引，重复回传不重复入库）
- [ ] 任务失败自动重试上限（3 次，指数退避，超限置 failed + 告警事件）
- [ ] Master 启动超时任务对账（30min 重置 processing→pending）
- [ ] 任务级超时上限实现（scan_policies.timeout 默认 60min，Master TaskScheduler 对账超时未完成中止置 failed；Worker 侧执行超时终止并在结果标记 task_timeout=true，见 R4.3-8）
- [ ] 断点续扫（local:crawled:{task_id}）
- [ ] Master/Worker 优雅停机（SIGTERM 排空在途任务 + Outbox 落盘，超时 30s）

### 1.6 证据链
- [ ] 证据 gzip 落盘 /data/evidence/{date}/ + MD5 去重 + SHA-256 入库
- [ ] 证据读取时 Hash 强制校验（不一致返回 EVIDENCE_TAMPERED）
- [ ] 通用证据读取接口（GET /api/v1/evidence/:id，返回 Req/Resp/HTML/截图元数据 + 文件流）
- [ ] 漏洞证据接口（GET /api/v1/vulnerabilities/:id/evidence，聚合漏洞关联证据链）
- [ ] Worker 侧证据生成（Req/Resp/HTML 快照/代码定位行号/confidence 置信度）+ 结果回传链路
- [ ] 结果回传协议实现（result_id 幂等 + status completed/failed/cancelled + task_timeout/stopped_by_user + findings 统一结构 + evidence_ids 数组引用 + 内联证据落盘，见 design「Worker 结果回传协议」）
- [ ] 证据传输协议（POST /api/v1/worker/evidence：<1MB 内联 / ≥1MB 分片 ≤8MB / upload_id 断点续传 / 收齐合并 SHA-256 校验）
- [ ] Worker HAR 文件生成（HAR 1.2 组装：entries 请求/响应头、Body、时间戳、大小、MIME + Body 超限截断标记 + gzip 落盘入库）
- [ ] 证据下载接口支持 format=har（HAR 文件导出，可导入 DevTools/Fiddler）
- [ ] 证据下载端点（GET /api/v1/evidence/:id/download 按证据类型返回文件流，下载前 Hash 校验）
- [ ] Worker 页面渲染截图采集（截图取证）
- [ ] Worker 无头浏览器截图组件（chromedp/chromium：viewport 渲染 + DOMContentLoaded+2s + PNG 输出，超时降级 screenshot:skipped，同 URL 缓存，并发受池约束）
- [ ] 截图上传接口（POST /api/v1/evidence/screenshots，Base64 或文件流，鉴权 + SHA-256 校验）
- [ ] 截图上传安全校验（MIME 仅 png/jpeg/webp + 大小 ≤10MB + 文件名防路径穿越/UUID 落盘）
- [ ] 证据文件保留期清理与空间回收（expires_at 365 天 + 孤儿文件扫描）
- [ ] 前端全屏证据抽屉（Req/Resp 分屏 + HTML 行号高亮 + 截图标签页 + 下载按钮；HTML 经 DOMPurify 白名单净化后渲染，禁止直接 v-html 注入防存储型 XSS）

### 1.7 仪表盘与报告
- [ ] 仪表盘后端接口（GET /api/v1/dashboard/stats 统计卡片 + trends 7 天趋势 + top-risks 风险 Top10 + engine-coverage 引擎覆盖率）
- [ ] 统计卡片 + 7 天趋势 + 风险 Top10
- [ ] 引擎覆盖率雷达图（10 大引擎检测覆盖率）
- [ ] WebSocket 实时事件滚动 + 指数退避重连 + 心跳保活（每 30s ping / 服务端 ReadDeadline 60s / 连续 3 次无 pong 触发重连，/api/v1/ws/events 订阅协议）
- [ ] ECharts 图表集成（雷达图/趋势图/折线图/可用性点阵图）
- [ ] 结构化日志 + request_id 请求追踪中间件 + 敏感字段脱敏（密码/Token/身份证/手机号/Headers）
- [ ] 报告导出（PDF 含水印 / Excel 漏洞清单）
- [ ] 前端 vitest 组件测试：登录表单校验/路由守卫/权限菜单渲染 + 证据抽屉 Hash 失败标红 + v-permission 按钮隐藏 + 证据 HTML XSS 净化（DOMPurify 注入 script/onerror/iframe 断言剥离）

### 1.8 阶段 1 单元测试【必执行】
- [ ] RBAC 权限矩阵单元测试（四角色 × 读写操作表驱动，按 design「RBAC 权限矩阵与权限码」逐模块断言，viewer 写操作 403）
- [ ] 认证服务单元测试（bcrypt 校验/JWT 签发/refresh token 换发/jti 黑名单/组织选择/登录锁定/禁用用户与禁用组织登录拦截）
- [ ] 证据服务单元测试（gzip 落盘/SHA-256 校验/MD5 去重/篡改检测）
- [ ] 任务调度单元测试（状态机流转/超时对账/断点续扫/任务级超时上限中止/stop 停止信号与 Worker cancelled 回传/CronScheduler 按时区定时触发与暂停/禁用跳过）

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
- [ ] 内容安全引擎（AI 文本分类 + 敏感词正则双判定 + 敏感信息规则集提取 + 篡改基线）
- [ ] 敏感信息规则集提取（rule_definitions 规则按 scope 分层匹配 request line/header/body，s_regex 过滤 + f_regex 提取，命中写 sensitive_info_hits + Bleve + findings 主命中；覆盖身份证/手机号/邮箱/JWT/Authorization/云凭证）
- [ ] 递归扫描与资产发现（max_depth 1-5/单站并发 2-32/静态文件与无效链接过滤/URL 归一化去重/发现资产写 assets 标注类型/进度经 GET /api/v1/tasks/:id/progress 实时上报，配置读取 scan_policies.scan_depth/concurrency_limit/allow_static/same_origin）
- [ ] 多端 UA 综合评估器（MultiUAAssessor：PC 随机 UA + 标准移动 UA + 微信内置浏览器 UA + 无头浏览器移动视口模拟四探针 + 基础/特征/场景三级加权评分 + SimHash DOM 相似度阈值 + SPA 空壳识别容错 + 结论分级与处置建议 + probe_failed 降权 + 各端快照证据链）
- [ ] 内容安全引擎接入 MultiUAAssessor（端级敏感词/敏感信息命中计入评分）
- [ ] AI 内容分类适配层（AIAdapter：endpoint/model/key 环境注入 + 超时/429 失败回退正则 + 结果缓存 + gobreaker 熔断）
- [ ] 暗链挂马引擎（检测器子单元：关键字整词匹配 + HTML 双通道[正则 URL 打分/DOM 结构 script/frame/link/form] + JS 高危函数与混淆手法/信息熵 + CSS 隐藏手法与危险链接 + 隐藏手法专项[零宽/同色/出屏/实体编码] + 自定义规则逐条匹配 + 无头浏览器动态检测[运行时样式判隐藏/动态链接打分，复用 R3.2b 组件] + 双 UA 对比）
- [ ] Webshell 检测引擎（路径枚举 + 特征码 + 流量特征）
- [ ] 钓鱼检测引擎（模板比对 + Levenshtein + 证书异常）
- [ ] 端口服务监测引擎（TCP SYN 扫描，非特权 Worker 降级 TCP Connect 全连接并标记 scan_mode:connect + Banner 指纹 + 高危暴露告警）
- [ ] 可用性监测引擎扩展（DNS 监控：解析 IP 变更/解析失败/劫持检测）
- [ ] 可用性监测引擎扩展（PING 监控：丢包率/延迟/ICMP 不可达告警）
- [ ] DNS 安全引擎（多节点解析对比 + 污染检测 + 子域名爆破）
- [ ] 信誉监测引擎（IP/域名威胁情报查询）
- [ ] 安全情报引擎（CVE/CNVD/CNNVD 订阅 + 资产影响匹配）

### 2.3 事件中心与漏洞管理
- [ ] 事件列表 + 状态流转（待处理→处理中→已关闭→已归档）+ 事件详情接口（GET /api/v1/events/:id）
- [ ] 发现处理链路（回传幂等→落 findings→降噪过滤→生成事件→漏洞聚合→告警生成→WS 广播，见 design「发现处理链路」）
- [ ] 引擎→事件类型映射表实现（vuln_scan→漏洞 / content_security→内容违规 / hidden_link→暗链挂马|木马|篡改 / webshell→Webshell / phishing→钓鱼 / availability→可用性异常 / port_service→端口暴露 / dns_security→篡改|漏洞 / reputation→信誉异常 / intelligence→情报预警，见 design 发现处理链路映射表）
- [ ] 降噪在事件生成时生效（白名单 IP/忽略类型/聚合窗口/风暴抑制，命中同时抑制告警与推送，规则变更不回溯）
- [ ] 事件批量状态流转（POST /api/v1/events/batch）
- [ ] 单事件状态流转（POST /api/v1/events/:id/status）
- [ ] 事件类型筛选（12 类）+ 降噪规则完整 CRUD（GET/POST /api/v1/noise-rules，PUT/DELETE /api/v1/noise-rules/:id）
- [ ] 独立告警接口（GET /api/v1/alerts 列表 + GET /:id 详情 + PATCH /api/v1/alerts/:id 处置 + POST /api/v1/alerts/batch 批量处置）
- [ ] 漏洞表与告警表迁移（vulnerabilities/alerts 独立表，见 design ER 图）
- [ ] 漏洞列表（GET /api/v1/vulnerabilities，等级/状态/引擎筛选）+ 漏洞详情接口（GET /api/v1/vulnerabilities/:id）+ 证据链抽屉接入
- [ ] 漏洞证据接口（GET /api/v1/vulnerabilities/:id/evidence）对接前端抽屉
- [ ] 漏洞批量接口（batch-ticket 批量生成工单 / batch-retest 批量复测 / batch-ignore 批量忽略）
- [ ] 漏洞复测流转（retest 置 verifying + 复测任务，通过自动 closed 写 closed_at，失败回退 open 追加复测记录，取消忽略恢复 open）
- [ ] 工单接口（GET/POST /api/v1/tickets 列表/创建 + PUT /api/v1/tickets/:id 状态/派发 + GET /api/v1/tickets/:id 详情）+ 工单闭环（确认→派发→修复→复测→归档）+ SOP 挂载
- [ ] 工单来源关联（tickets.event_id 事件来源 / vuln_id 漏洞来源，至少其一非空，漏洞批量生成工单可关联）
- [ ] SOP 模板库（内置按事件类型分组应急响应 SOP + 事件确认自动挂载 sop_attached + org_admin 自定义）
- [ ] 独立告警中心前端页（列表/等级筛选/处置操作/静默 + 导航未读角标联动）
- [ ] 告警处置三态流转（确认/关闭/静默 + 批量，静默抑制该资产同类通知与重新告警 + 可恢复 open，区别于 noise_rules，见 design 告警处置语义）
- [ ] 漏洞管理前端页（等级/状态/引擎筛选 + 详情 + 证据链抽屉 + 批量生成工单/复测/忽略）
- [ ] 工单前端页（列表/详情/状态流转/SOP 挂载/复测归档）
- [ ] 告警风暴抑制（单资产每小时 5 条上限）

### 2.4 引擎相关前端模块
- [ ] 内容安全监测页（敏感内容/信息泄漏/篡改对比 + 截图缩略图 + 多端 UA 评估结果对比明细/评分/端级异常定位 + 敏感信息规则集管理视图 group/name/scope/sensitive 启用禁用 + 资产发现结果展示 JS/CSS/图片/音视频/子域名/接口路径 + 递归扫描进度展示）
- [ ] 暗链木马页（暗链列表/木马列表含检测维度来源标注 + 双 UA 对比 + 双 UA 触发接口 POST /api/v1/assets/:id/dual-ua）
- [ ] Webshell 与钓鱼页（Webshell 列表：检测路径/特征码/文件内容片段 + 钓鱼列表：仿冒目标/域名相似度/证书异常）
- [ ] 可用性网络页（12h 点阵图绿/红竖线 + HTTP/DNS/PING 三维度切换 + 24h 时序折线 + DNS/端口记录 + 多端 UA 可用性对比与端差异化异常展示）
- [ ] 时序查询后端接口（GET /api/v1/assets/:id/availability 点阵图 + /response-time 折线，读 availability_points 按 org_id 隔离）
- [ ] 安全情报中心页（情报列表 + 受影响资产数 + 订阅配置 GET/PUT /api/v1/intel-subscriptions）
- [ ] 情报后端接口（GET /api/v1/intel 列表筛选分页 + GET /:id 详情含受影响资产）
- [ ] 任务队列监控页（排队/处理中/完成 + Worker 分配 + 断点续扫状态）
- [ ] 全局搜索入口（顶部导航搜索框 + 结果页分类展示资产/发现/事件，走 /api/v1/search）

### 2.5 重组件集成
- [ ] Bleve 索引 + BatchIndexer（5s/50 条批量提交）
- [ ] 全局搜索接口（GET /api/v1/search：跨 assets/findings/events 全文检索 + org 隔离 + 分页）
- [ ] Bleve 删除同步（删除/级联清理时 batch 按 id 删索引 + index rebuild 全量重建命令）
- [ ] BadgerDB 元数据缓存层（API 毫秒级响应）
- [ ] 异步批量持久化 SQLite/Bleve 通道

### 2.6 阶段 2 单元测试【必执行】
- [ ] 引擎契约单元测试（10 引擎 mock 输入 → finding 输出）
- [ ] 敏感信息规则提取单测（scope 分层命中/凭证规则/递归去重/静态文件过滤/深度上限）
- [ ] 发现处理链路单元测试（回传幂等 → 降噪过滤命中丢弃 → 事件生成 → 漏洞聚合首建/更新 last_seen_at → 告警生成按 severity 阈值与通知路由 → WS 广播）
- [ ] AI 适配层单元测试（AI 可用→ai 来源 / AI 超时/429→regex 回退 / 熔断切换）
- [ ] MultiUA 评估器单元测试（四探针抓取对比 / 三级加权评分 / 端差异化宕机 / 移动端定向投毒 / 微信 UA 单独放行 / SimHash 相似度>90% 不加分 / SPA 空壳不覆盖 / probe_failed 降权 / 结论分级）
- [ ] 脱敏单元测试（身份证/手机号/邮箱/AccessKey 三时机脱敏）
- [ ] Worker 握手单元测试（Bootstrap Token 一次性使用/过期拒绝、长期凭证校验、凭证吊销后拒绝、token/secret 落库 hash 无明文，对应 design「Test Strategy → Worker 握手」）
- [ ] Bleve 索引单元测试（删除/级联清理同步删索引、index rebuild 全量重建对账）

### 阶段 2 验收
- [ ] 10 大引擎全部可开关执行【必执行】
- [ ] 事件/漏洞/工单全流程闭环【必执行】
- [ ] 万级列表虚拟滚动性能达标【必执行】
- [ ] go test ./... 全部通过 + 前端 vue-tsc + vite build 通过【必执行】

---

## 阶段 3 — 全量功能 + 平台化

### 3.1 团队与系统设置
- [ ] 成员管理（GET /api/v1/members 列表 + POST 单条邀请/批量邀请 batch-invite + 移除 DELETE :id/批量移除 batch-remove + 禁用/启用 :id/disable + 修改角色 PUT :id）— 仅 org_admin
- [ ] 受邀成员首次登录激活（invited → active，强制设密码/改密，邀请链接 7 天过期）
- [ ] Worker 节点管理（心跳/负载/版本/Bootstrap Token + 状态 online/offline/offline_removed 判定：心跳超 3 倍置 offline，移除置 offline_removed 不计配额，调度仅向 online 分发 + 移除离线节点 DELETE /api/v1/worker/nodes/:id）
- [ ] 通知渠道配置完整 CRUD（GET/POST + PUT/DELETE :id，钉钉/企微/飞书 Webhook + SMTP 多渠道 + 按 id 测试 POST :id/test）
- [ ] 通知渠道密钥加密（AES-256-GCM 主密钥 CINSIGHT_CHANNEL_KEY + 接口掩码脱敏 + 编辑留空保持原值）
- [ ] 通知路由规则（GET/PUT /api/v1/notify-routes：rule JSON 匹配 severity/event_type 优先 event_type 后 severity，* 通配 + default_channel_id 兜底 + 渠道启用开关 + 风暴抑制在路由层生效）
- [ ] Worker 注册握手配额校验（已注册 Worker ≥ max_workers 返回 4291 WORKER_QUOTA_EXCEEDED，删除节点释放配额）
- [ ] 规则库管理（POC/敏感词/木马特征库/敏感信息规则集 + 版本号 + 规则项增删改查 GET/POST /api/v1/rules/items + PUT/DELETE /api/v1/rules/items/:id + 导入 GET/POST /api/v1/rules/import（敏感信息规则集支持 HaENet Rules.yml YAML）+ 导出 /api/v1/rules/export）
- [ ] 情报订阅配置独立接口（GET/PUT /api/v1/intel-subscriptions，CVE/CNVD/CNNVD 数据源开关）
- [ ] 审计日志（禁止修改删除 + 筛选 operator/action/resource_type/start/end + 分页 + 服务端捕获 IP/User-Agent + action 统一 resource.verb 枚举见 design audit_logs 段 + 覆盖范围见 design audit_logs 段，批量逐条记录，读操作与引擎回传不审计）
- [ ] API Token 管理（GET/POST 列表/创建，scopes 取 RBAC 权限码子集勾选 + 有效期 + 撤销 DELETE :id + 停用/恢复 PATCH :id/status，校验时接口所需权限码须为 token scopes 子集，无 scope 返回 2101 SCOPE_DENIED，scopes 不可改需撤销重建）
- [ ] 团队管理前端页（成员列表/邀请/批量移除/禁用/改角色）
- [ ] 系统设置前端页（Worker 节点管理/通知渠道 CRUD+测试/通知路由规则/规则库管理/API Token/Webhook/审计日志筛选）

### 3.2 平台管理
- [ ] 组织列表/创建/详情/编辑/禁用/启用/删除（GET/POST /api/v1/orgs + GET/PUT/DELETE /api/v1/orgs/:id + POST :id/disable + :id/enable，DELETE 需输入组织名二次确认 + 级联清理；enable 恢复 cron 与写操作）— 仅 super_admin
- [ ] 组织配额/套餐限制校验（创建资产超 max_assets → 4290 ASSET_QUOTA_EXCEEDED；成员超 max_members → 4292 MEMBER_QUOTA_EXCEEDED；Worker 超 max_workers → 4291 WORKER_QUOTA_EXCEEDED；org 详情返回 used_assets/used_workers/used_members）
- [ ] 组织到期/禁用行为（停止 cron 计划 + 拒绝新建任务与资产写操作 + 仅保留只读；到期前 7 天续费提示）
- [ ] 平台统计（总组织/总资产/总扫描/总事件）
- [ ] 平台 Worker 总览
- [ ] 平台管理前端页（组织列表 CRUD + 配额/到期展示 + 平台统计 + Worker 总览，仅 super_admin 入口）

### 3.3 集成与部署
- [ ] swaggo Swagger 文档全量注解
- [ ] API Token 认证中间件（独立于 JWT，scopes 细粒度 + 有效期）
- [ ] Webhook 完整 CRUD（GET/POST + PUT/DELETE :id + 测试推送 POST :id/test + 签名密钥重新生成 POST :id/secret + Secret 一键复制）
- [ ] Webhook 事件推送（HMAC-SHA256 签名 + 重试 3 次 + 落库）
- [ ] Webhook 订阅事件枚举过滤（events 数组订阅：finding.*/vulnerability.*/event.*/alert.*/task.*/intel.high，未命中订阅不推送，见 design Webhook 订阅事件枚举）
- [ ] 规则热更新（fsnotify）+ Worker 规则 Hash 同步
- [ ] 扫描授权拦截（白名单校验 + 内网 IP 禁止）+ 白名单管理接口（GET/PUT /api/v1/scan-whitelist + Worker Hash 同步）
- [ ] gobreaker 目标熔断（连续失败 5 次）+ 授权违规熔断 Worker
- [ ] 三时机脱敏（入库/API 返回/报告生成）
- [ ] 反封禁（Proxy 配置 + 低速隐蔽模式）
- [ ] VictoriaMetrics 迁移接口预留（MetricsExporter，当前 SQLite 时序表）
- [ ] 批量操作规范落地（POST /{resource}/batch：{ids} ≤500，返回 {success,failed}，幂等）

### 任务 15 — Litestream 热备与部署【必执行】
- [ ] Litestream SQLite 实时流式热备（配置 + 复制验证）
- [ ] Docker 编排（Master + Worker + SQLite 卷 + 数据目录挂载 + Worker 镜像含 chromium 无头浏览器依赖）
- [ ] K8s 编排（Master 水平扩展读写分离 + Worker 弹性伸缩 HPA）
- [ ] 私有化单二进制一键安装打包（零外部依赖）
- [ ] 部署验证：Docker 起服务 → 探活 /api/health → 建资产 → 下发任务全链路通过

### 3.4 报告中心全量
- [ ] 策略模板完整 CRUD（GET/POST /api/v1/policies + PUT/DELETE /api/v1/policies/:id + 批量删除 POST /api/v1/policies/batch-delete；引擎开关/并发/超时/速率限制/递归扫描参数维护，engineer 读用于任务创建选模板）
- [ ] 报告模板完整 CRUD（执行摘要/漏洞详情/内容安全/可用性统计/整改建议 + PUT/DELETE :id）
- [ ] Cron 定时计划（绑定资产分组 + 策略模板 + 时间窗口 + 时区 CINSIGHT_TIMEZONE 默认 Asia/Shanghai + 完整 CRUD PUT/DELETE :id + 启停开关 PATCH :id/status + 批量启停 batch-toggle）
- [ ] Master CronScheduler 计划调度（按 cron_expr+计划时区定时生成 scan_tasks 入队分发；time_window 执行时间窗口窗口外跳过/顺延；paused/组织禁用/到期跳过触发；组织启用后恢复触发）
- [ ] 策略模板复制（POST /api/v1/policies/:id/copy 深拷贝引擎开关）
- [ ] 定时报告（模板 cron_expr+timezone 配置、Cron 生成周报/月报 + 时区 CINSIGHT_TIMEZONE + 异步生成进度条 + 完成通知）
- [ ] 报告生成基于生成时刻数据快照（漏洞/发现/可用性时点固化，后续处置不影响已生成报告）
- [ ] 报告截图合集导出（按资产/时间范围，format=screenshots 下载 ZIP）
- [ ] 报告详情（GET /api/v1/reports/:id）+ 报告删除（DELETE /api/v1/reports/:id）

### 3.5 前端 UX 基座（R5.19）
- [ ] 全局 Toast + MessageBox 二次确认弹窗封装 + 全局错误边界组件（R5.19-1）
- [ ] 通用表格基座：多选批量操作栏/分页/排序/筛选重置/日期选择/空态/骨架屏/搜索防抖 300ms（R5.19-2）
- [ ] 统一表单校验规则库 + 新建/编辑共用抽屉 + 保存中禁用防重复提交（R5.19-3）
- [ ] 图表基座：ECharts 统一配色 + 阈值配色 + 角色 Tag 颜色规范（R5.19-4）
- [ ] 通用详情抽屉基座（Req/Resp 分屏 + HTML 行号高亮 + 截图 tab + 下载 + 时间线，R5.19-5）
- [ ] 各业务页批量操作对齐：资产/事件/告警/漏洞/任务/成员/策略/计划（R5.19-6 + R5.17-13 批量规范）

### 3.6 前端体验统一（UX）
- [ ] 列表空状态引导（插画 + 文案 + 主操作按钮）
- [ ] 危险操作二次确认弹窗（删除/批量删除/移除成员/撤销 Token/删除组织需输入名称）
- [ ] 批量操作栏（多选后浮动出现：已选 N 项/全选/跨页记忆 + 结果汇总 Toast + 失败明细展开）
- [ ] 一键复制组件（资产 URL/API Token/Webhook Secret/Bootstrap Token）
- [ ] 行内快捷操作（立即扫描/生成工单/申请复测/告警确认）
- [ ] 筛选条件持久化（localStorage）+ URL 参数分享
- [ ] 导航栏事件/告警未读角标（WebSocket 实时更新）
- [ ] 任务详情页（进度条/执行日志/结果统计/Worker 分配）
- [ ] 资产 URL/CSV 批量导入前端（模板下载 + 逐行校验报告展示）+ 当前筛选结果 CSV 导出
- [ ] 报告异步生成进度 + 完成通知
- [ ] 相对时间展示 + 状态色统一（高危红/中危橙/低危黄/正常绿）

### 阶段 3 验收
- [ ] 全部 15 个功能模块上线【必执行】
- [ ] 平台管理/团队管理权限隔离正确【必执行】
- [ ] 组织配额单元测试（资产超 max_assets → 4290 / 成员超 max_members → 4292 / Worker 超 max_workers → 4291 + 批量逐条 failed + 到期组织写操作拒绝）
- [ ] 通知渠道密钥加密单元测试（AES-256-GCM 加解密 + 返回掩码脱敏 + 留空保持原值）
- [ ] 通知路由单元测试（severity/event_type 命中映射 → 指定渠道 / 未命中 → 默认渠道 / 渠道禁用跳过 / 风暴抑制在路由层生效）
- [ ] 审计日志筛选单元测试（operator/action/resource_type/时间范围过滤 + 分页）
- [ ] 集成测试：Master + 单 Worker 全链路（任务下发→引擎执行→结果回传→证据入库→前端展示）【必执行】
- [ ] 前后端 CRUD 与批量接口对齐验收（逐模块 Create/Read/Update/Delete/Batch 全覆盖，前端按钮与后端端点一一对应）【必执行】
- [ ] 容灾演练：Worker 断网恢复（Outbox 回传）、Master 重启对账、熔断触发【必执行】
- [ ] 部署验证：Docker/K8s/单二进制三种方式启动 + 探活 + 全链路通过【必执行】
- [ ] go test ./... 全部通过 + 前端 vue-tsc + vite build 通过【必执行】

---

## 阶段 4 — 企业级工程化与合规达标【必执行】

### 4.1 可观测性
- [ ] Prometheus 指标端点（GET /metrics，RED + USE + 业务指标 + 敏感信息/资产发现指标）
- [ ] 健康探针（GET /healthz 存活 + GET /readyz 就绪：SQLite/Badger/证据目录/Litestream）
- [ ] OpenTelemetry 分布式追踪（trace_id 贯穿 Master→Worker→外部调用，采样 10%）
- [ ] SLO 监控面板（API 可用性 ≥ 99.9%、p99 < 500ms、任务成功率 ≥ 99%）

### 4.2 安全加固
- [ ] TLS 终结（网关）+ HSTS + 安全响应头中间件（CSP/X-Frame-Options/nosniff/Referrer-Policy）
- [ ] API 通用限流（每用户/IP 100 req/min，超限 HTTP 429 + Retry-After 头）+ 登录接口独立限流（5 次/min/IP，连续失败 5 次锁 15 分钟）
- [ ] 密码策略（≥12 位复杂度/90 天轮换/禁复用 5 次/首登强制改密）
- [ ] WebSocket 越权订阅防护（握手校验 JWT + org_id，通道绑定 org，禁止跨组织订阅）
- [ ] 乐观锁并发控制（assets/scan_policies/alerts/tickets 含 version，不匹配返回 409）
- [ ] MFA 预留（TOTP 二次认证开关）
- [ ] Secrets 管理（环境/K8s Secret 注入 + 轮换，禁止入日志）
- [ ] 依赖漏洞扫描（govulncheck）+ 容器镜像扫描（Trivy）+ distroless 最小化镜像

### 4.3 CI/CD 流水线
- [ ] CI：golangci-lint + go vet → go test -race -cover → 单二进制构建 → 前端 typecheck+build
- [ ] CD：多阶段镜像构建 → 安全扫描 → 蓝绿/金丝雀部署 dev/staging/prod
- [ ] 质量门禁：核心包行覆盖 ≥ 80%，lint 零错误，依赖漏洞零 high/critical
- [ ] SemVer 版本管理 + CHANGELOG 自动生成 + git flow 分支策略
- [ ] 多环境配置分离（dev/staging/prod + .env.example）

### 4.4 高可用与数据治理
- [ ] RPO/RTO 指标（Litestream RPO ≤ 5s、RTO ≤ 30min、备份保留 30 天）
- [ ] Master 只读副本水平扩展（查询路由副本 + 单写通道）
- [ ] 数据保留与冷归档（事件/漏洞/告警 180 天热数据 + 审计日志 365 天 + archived=true 只读查询路由 + 写操作拒绝）
- [ ] 证据文件保留期清理与空间回收（expires_at + 孤儿文件扫描）
- [ ] 备份恢复与演练脚本（backup.sh/restore.sh/drill.sh，验证数据完整性与可启动性）
- [ ] 季度恢复演练 + 容量规划验证（10 万资产/100 Worker/1000 任务并发）

### 4.5 测试完备性
- [ ] E2E 测试（Playwright：登录→组织切换→建资产→任务→证据抽屉→报告导出）
- [ ] 集成测试：认证→资产→任务→结果→事件→闭环全链路（含幂等键去重/乐观锁/WS 越权拒绝/截图上传校验）
- [ ] k6 压测脚本（登录并发/资产列表/事件写入/证据读取）验证 SLO
- [ ] 故障注入演练（磁盘写满/网络延迟抖动/Litestream 中断/Worker 断网）

### 4.6 前端工程化与合规
- [ ] 路由守卫（beforeEach：token/org/role 校验）
- [ ] 全局错误处理（axios 拦截器 + 异常兜底页 + 骨架屏 + 请求失败 Toast + WS 断线提示条）
- [ ] i18n（vue-i18n 中/英）+ 可访问性（键盘可达/ARIA）
- [ ] 路由懒加载 + 组件分包，首屏 < 3s
- [ ] 账户注销 + 个人信息删除/匿名化（PIPL/GDPR）
- [ ] 个人数据可携权导出（GET /api/v1/me/data-export：JSON/CSV + 保留 72h + 审计记录）
- [ ] 等保 2.0 对齐（身份鉴别/访问控制/安全审计/数据完整保密）

### 阶段 4 验收
- [ ] /metrics + /healthz + /readyz 探活通过【必执行】
- [ ] CI/CD 流水线全绿（lint→test→build→scan→deploy）【必执行】
- [ ] E2E 关键路径 + k6 压测满足 SLO【必执行】
- [ ] 恢复演练 + 故障注入演练报告【必执行】
- [ ] go test ./... 全部通过 + 前端 vue-tsc + vite build 通过【必执行】
