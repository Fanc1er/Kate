# CInsight 智能监测平台 — 需求文档

## Introduction

CInsight 是一个工业级 SaaS 多租户安全监测平台，采用 Master-Worker 分布式架构，内置 10 大检测引擎（漏洞扫描、内容安全、暗链挂马、Webshell、钓鱼、可用性、端口服务、DNS 安全、信誉监测、安全情报），为租户提供资产持续监测、漏洞发现、内容合规、证据取证与闭环处置能力。

平台遵循三项核心原则：
- **零依赖理念**：单二进制一键部署，各引擎模块化、可插拔。
- **Master-Worker 模型**：Master 单写 SQLite 持久化，Worker 仅拉取任务并回传结果，天然支持弹性伸缩。
- **全量证据链**：每项发现关联可验证的请求/响应/HTML 快照/代码定位/截图证据，SHA-256 防篡改。

## Glossary

- **Master**：中心节点，负责 API、任务调度、持久化、证据落盘、规则管理。
- **Worker**：执行节点，拉取任务、运行检测引擎、回传结果。
- **租户（Org）**：平台内的独立组织，数据按 `org_id` 强制隔离。
- **证据（Evidence）**：检测结论的支撑材料（HTTP Req/Resp、HTML 快照、代码片段、截图）。
- **引擎（Engine）**：Worker 内的独立检测模块，实现一类检测能力。
- **任务（Task）**：一次对资产集合执行策略的检测调度单元。
- **事件（Event）**：引擎检测出的待处置发现，支持状态流转与闭环。
- **工单（Ticket）**：事件处置过程中的任务单，承载修复跟踪。
- **Outbox**：Worker 本地断网缓存，网络恢复后批量回传结果的机制。

## Requirements

### 1. 多租户与 RBAC 权限体系

#### R1.1 身份认证

**User Story:** AS 任意用户，I want 使用用户名密码登录并获得 JWT，SO THAT 平台能识别我的身份与角色。

**Acceptance Criteria:**
1. WHEN 用户在统一登录页提交用户名与密码，系统 SHALL 用 bcrypt 校验密码并在校验通过后签发 JWT，JWT 包含 `user_id` 且不含 `org_id`。
2. WHEN JWT 签发完成，系统 SHALL 查询 `user_orgs` 关联表：单组织用户直接返回 `org_id` 与 `role`；多组织用户返回组织列表由前端展示选择卡片。
3. WHEN 多组织用户通过 `POST /api/v1/auth/select-org` 提交目标组织，系统 SHALL 校验该用户在此组织的有效成员关系并签发携带 `org_id` 的新 JWT。
4. WHEN 用户请求受保护 API，系统 SHALL 校验请求头 `Authorization: Bearer {jwt}` 与 `X-Org-Id: {org_id}`，任一项缺失或无效时返回 401。
5. IF 密码连续校验失败达到锁定阈值，系统 SHALL 临时锁定该账户并返回友好提示。
6. WHEN 用户退出登录，前端 SHALL 清除本地 JWT 并跳转登录页。

#### R1.2 四层角色与权限矩阵

**User Story:** AS 平台，I want 按角色控制功能访问，SO THAT 只读用户无法写入、普通组织成员无法管理平台。

**Acceptance Criteria:**
1. 系统 SHALL 定义四层角色：`super_admin`（平台超管）、`org_admin`（组织管理员）、`engineer`（安全工程师）、`viewer`（只读用户）。
2. WHEN 角色为 `viewer` 的用户发起任何写操作 API，系统 SHALL 拒绝并返回 403，只读用户严禁写入。
3. WHEN 角色为 `super_admin` 的用户访问平台总览与组织管理，系统 SHALL 授予全部组织范围访问权（通过全局 `org_id=0` 查询）。
4. WHEN 角色为 `org_admin` 的用户访问成员管理、Worker 节点管理、策略配置、通知渠道、审计日志，系统 SHALL 授予本组织范围权限。
5. WHEN 角色为 `engineer` 的用户访问资产、漏洞、告警、任务、报告，系统 SHALL 授予本组织范围操作权限。
6. WHEN 任意角色访问仪表盘与报告导出，系统 SHALL 授予查看权限（报告导出对所有角色开放）。
7. 系统 SHALL 通过 RBAC 中间件在路由层强制校验角色权限，所有写操作 API 必经 RBAC 校验。

#### R1.3 前端动态路由与组织切换

**User Story:** AS 前端，I want 根据登录返回的 role 动态生成菜单与路由，SO THAT 用户只能看到有权限的模块。

**Acceptance Criteria:**
1. WHEN 用户登录成功，前端 SHALL 依据 `role` 通过 `addRoute()` 动态注册路由：`org_admin` 含仪表盘/资产/漏洞/告警/任务/内容安全/报告/团队管理/系统设置；`engineer` 含仪表盘/资产/漏洞/告警/任务/内容安全/报告；`viewer` 含仪表盘/资产/漏洞/告警/报告。
2. WHEN 用户为 `super_admin`，前端 SHALL 展示"进入平台管理"与"选择组织"两个入口。
3. WHEN 顶部导航栏加载，系统 SHALL 展示当前组织名 + 用户角色 Tag，并支持多组织用户点击切换组织。
4. WHEN 用户切换组织，前端 SHALL 调用 `POST /api/v1/auth/select-org` 换取新 JWT 并刷新全部数据请求。

#### R1.4 核心数据模型

**User Story:** AS 平台，I want 用核心表承载组织、用户、成员关系，SO THAT 多租户隔离有数据基础。

**Acceptance Criteria:**
1. 系统 SHALL 维护 `organizations` 表（id, name, logo_path, plan, max_assets, max_workers, expire_at, status）。
2. 系统 SHALL 维护 `users` 表（id, username, password(bcrypt), email, phone, status, last_login_at, is_super_admin）。
3. 系统 SHALL 维护 `user_orgs` 关联表（user_id, org_id, role, status, joined_at），并禁止 `super_admin` 加入 `user_orgs`。
4. 所有业务查询 SHALL 强制附带 `org_id` 过滤条件，未携带则拒绝。

### 2. 十大检测引擎（Worker 核心）

#### R2.1 通用引擎契约

**User Story:** AS Worker，I want 统一的引擎注册与任务执行契约，SO THAT 各引擎可插拔开关。

**Acceptance Criteria:**
1. Worker SHALL 以模块化方式内置 10 大检测引擎，各引擎实现统一接口（名称、开关、配置、执行函数）。
2. 每次 POC 执行 SHALL 使用 `context.WithTimeout(30s)` 控制超时，使用 `ants` 协程池控制并发。
3. 引擎执行 SHALL 遵循策略模板的开关、并发数、超时与速率限制。
4. 引擎产生的每项发现 SHALL 附带证据链并统一回传 Master。

#### R2.2 漏洞扫描引擎

1. WHEN 收到扫描任务，引擎 SHALL 执行 POC 漏洞检测、Fuzzing 测试与参数注入检测。
2. 引擎 SHALL 对每个 POC 建立超时上下文，防止单请求阻塞协程池。

#### R2.3 内容安全引擎

1. WHEN 检测页面内容，引擎 SHALL 通过 AI 文本分类 + 关键词正则双判定识别涉黄/涉赌/涉毒/涉政内容。
2. WHEN 检测敏感信息，引擎 SHALL 识别身份证号、手机号、邮箱、AccessKey 泄漏并记录命中位置。
3. WHEN 监控页面篡改，引擎 SHALL 以标题、图片 Hash、正文 DOM 结构三维度建立基线，WHEN 偏离阈值超过设定值，系统 SHALL 生成篡改告警。

#### R2.4 暗链挂马引擎

1. WHEN 检测暗链，引擎 SHALL 识别隐藏外链、友情链接、SEO 黑帽链接。
2. WHEN 检测木马，引擎 SHALL 匹配网页木马特征库并分析 iframe 嵌套、JS 混淆、Shellcode 等沙箱行为。
3. WHEN 执行双 UA 对比，引擎 SHALL 切换正常 UA 与蜘蛛 UA 抓取页面，检测条件性加载的暗链。

#### R2.5 Webshell 检测引擎

1. WHEN 检测 Webshell，引擎 SHALL 执行常见路径枚举（如 /uploads/shell.php）。
2. 引擎 SHALL 对文件内容匹配特征码（eval、gzinflate、assert 等）。
3. 引擎 SHALL 识别异常 POST 请求模式等流量特征。

#### R2.6 钓鱼检测引擎

1. WHEN 检测钓鱼，引擎 SHALL 比对页面结构、Logo、Form 动作与已知钓鱼模板。
2. 引擎 SHALL 通过 Levenshtein 距离计算目标域名与品牌域名的相似度。
3. 引擎 SHALL 检测 SSL 证书签发机构与有效期异常。

#### R2.7 可用性监测引擎

1. WHEN 执行 HTTP 监控，引擎 SHALL 记录状态码、响应时间、页面大小，IF 连续失败 3 次，系统 SHALL 判定宕机并告警。
2. WHEN 执行 DNS 监控，引擎 SHALL 检测解析 IP 变更、解析失败与 DNS 劫持。
3. WHEN 执行 PING 监控，引擎 SHALL 记录丢包率与延迟，IF ICMP 不可达，系统 SHALL 告警。

#### R2.8 端口服务监测引擎

1. WHEN 执行端口扫描，引擎 SHALL 以 TCP SYN 方式扫描 Top 常见端口（22/21/3389/3306/6379 等）。
2. 引擎 SHALL 通过 Banner 抓取识别服务类型与版本。
3. WHEN 检测到新增开放端口或高危服务暴露（如 Redis 6379 对外开放），系统 SHALL 生成告警。

#### R2.9 DNS 安全引擎

1. WHEN 检测 DNS 劫持，引擎 SHALL 对比多节点解析结果的一致性。
2. WHEN 检测 DNS 污染，引擎 SHALL 识别异常 TTL 与异常解析记录。
3. 引擎 SHALL 通过字典爆破 + 被动 DNS 收集发现子域名。

#### R2.10 信誉监测引擎

1. WHEN 检测 IP 信誉，引擎 SHALL 查询目标 IP 在威胁情报库中的恶意/代理/Tor 标记。
2. WHEN 检测域名信誉，引擎 SHALL 检查域名历史解析记录与恶意标记状态。

#### R2.11 安全情报引擎

1. WHEN 情报源更新，引擎 SHALL 订阅 CVE/CNVD/CNNVD 漏洞情报并自动匹配资产影响范围。
2. WHEN 收到高危情报（如 0day），系统 SHALL 自动创建告警并推送通知。

### 3. 全量证据链与防篡改

#### R3.1 证据内容采集

**User Story:** AS 安全工程师，I want 每个漏洞/告警有完整证据链，SO THAT 可追溯可取证。

**Acceptance Criteria:**
1. 每个漏洞/告警 SHALL 关联证据链，包含请求信息（HTTP 方法、URL、Headers、Body）、响应信息（状态码、Headers、Body）、HTML 源码快照、代码位置片段（精确到行号）与截图取证。
2. 引擎 SHALL 输出漏洞触发点的关键代码片段与行号（如 `<script>alert(1)</script>` 在第 152 行）。

#### R3.2 证据存储与校验

1. 大体积证据数据（HTML 源码、完整响应体）SHALL 经 gzip 压缩落盘至 `/data/evidence/{date}/`，数据库仅存元数据、文件路径与 SHA-256 签名。
2. 证据文件 SHALL 通过 MD5 去重，SHA-256 签名入库。
3. WHEN 前端展示证据，后端 SHALL 强制校验文件 Hash，IF 校验不一致，UI SHALL 标红提示"证据已被破坏"。

#### R3.3 前端证据展示

1. 前端 SHALL 提供全屏抽屉，上方左右分屏展示 HTTP Req/Resp。
2. 下方 SHALL 展示 HTML 源码（代码高亮 + 行号标红定位）。
3. 前端 SHALL 提供截图取证标签页展示渲染截图，并提供下载完整 HTML 快照 / HAR 文件按钮。

#### R3.4 截图上传接口

**User Story:** AS 取证人员，I want 上传页面渲染截图，SO THAT 证据链包含可视化取证材料。

**Acceptance Criteria:**
1. 系统 SHALL 提供 `POST /api/v1/evidence/screenshots` 上传接口，接受 Base64 或文件流。
2. 上传接口 SHALL 经鉴权（JWT/API Token）并校验文件类型与大小，上传后 SHALL 经 gzip 落盘并计算 SHA-256 入库。
3. 截图 SHALL 关联目标资产或证据记录，可在证据抽屉截图标签页展示。

### 4. 工程化与极限容灾

#### R4.1 目录与文档规范

1. 后端 SHALL 采用目录：`cmd/(master|worker)`、`internal/master/{controller,service,repository,middleware,routes}`、`internal/worker/{engine,scheduler,reporter}`、`pkg/{db,badger,bleve,storage,utils}`、`docs/swagger`。
2. API 文档 SHALL 集成 swaggo 注解自动生成 Swagger，暴露 `/swagger/*` 端点。
3. 前端 SHALL 基于 RBAC 权限表懒加载页面组件。

#### R4.2 极限性能

1. Master SHALL 使用 BadgerDB 仅存元数据，接收结果后异步批量持久化 SQLite/Bleve，保证 API 毫秒级响应。
2. Bleve 索引 SHALL 引入 BatchIndexer，后台协程每 5 秒或凑齐 50 条批量提交。
3. 万级列表 SHALL 启用虚拟滚动；WebSocket SHALL 断线指数退避重连。
4. 系统 SHALL 执行告警风暴抑制：单资产每小时通知上限 5 条，超出部分静默入库并追加高频提示。

#### R4.3 高可用容灾

1. Worker 断网时 SHALL 将结果缓存至本地 Outbox，网络恢复后批量回传。
2. Worker SHALL 以 `local:crawled:{task_id}` 记录已爬取 URL，重启后跳过已扫页面实现断点续扫。
3. Master 启动时 SHALL 将超时（30min）未回传的 `processing` 任务重置为 `pending`。
4. Master SQLite SHALL 通过 Litestream 实时流式备份。

#### R4.4 深度安全对抗

1. 资产 URL 入库前 SHALL 强制标准化（协议/默认端口/路径），BadgerDB 维护 MD5 防重。
2. 扫描 SHALL 执行授权拦截：白名单校验、内网 IP 禁止，IF 违规 3 次，系统 SHALL 自动熔断 Worker。
3. 系统 SHALL 预留 Proxy（HTTP/SOCKS5）配置与低速隐蔽模式（并发=1、伪造 UA/Referer）反封禁设计。
4. 目标请求 SHALL 经 `gobreaker` 熔断，IF 连续失败 5 次，系统 SHALL 熔断该目标。
5. 身份证、手机号等敏感数据 SHALL 在入库前、API 返回前、报告生成时三时机自动脱敏。
6. Master SHALL 通过 `fsnotify` 监听规则文件热加载，Worker SHALL 定期拉取 Hash 同步规则库。

### 5. 功能模块需求

#### R5.1 仪表盘（全部角色可见）

1. 系统 SHALL 展示统计卡片（资产总数/可用率/高危漏洞/待处理事件）。
2. 系统 SHALL 展示 7 天趋势图（漏洞发现/事件趋势）、资产风险 Top10 排行、实时事件 WebSocket 滚动。
3. 系统 SHALL 展示引擎覆盖率雷达图（10 大引擎检测覆盖率）。

#### R5.2 资产管理

1. 系统 SHALL 提供资产列表（虚拟滚动/模糊搜索/按重要程度/分组/状态筛选）。
2. 系统 SHALL 提供资产 CRUD（表单含 URL 归一化/名称/分组/重要程度/备注）。
3. 系统 SHALL 提供资产画像抽屉（技术栈指纹/ICP 备案/子域名/SSL 证书倒计时/端口服务快照）。
4. 系统 SHALL 记录变更追踪（标题/技术栈/状态码/端口变动历史）。
5. 系统 SHALL 支持微信公众号资产，字段包含公众号名、微信号、头像、粉丝数、简介、认证状态与文章数。

#### R5.3 安全事件中心

1. 系统 SHALL 提供事件列表（级别/来源资产/引擎类型/内容/状态流转：待处理→处理中→已关闭→已归档）。
2. 系统 SHALL 支持事件类型筛选：漏洞/内容违规/暗链挂马/木马/Webshell/钓鱼/篡改/可用性异常/端口暴露/敏感信息泄漏/信誉异常/情报预警。
3. 系统 SHALL 提供降噪规则配置（白名单 IP/忽略特定类型/聚合时间窗/风暴抑制）。
4. 系统 SHALL 支持闭环处置流程：事件确认→工单派发→修复跟踪→复测验证→归档，并自动挂载应急响应 SOP。

#### R5.4 漏洞与风险管理

1. 系统 SHALL 提供漏洞列表（按等级/状态/引擎类型筛选）。
2. 系统 SHALL 提供证据链抽屉（Req/Resp 分屏 + HTML 行号高亮 + 截图取证）。
3. 系统 SHALL 支持闭环处置（生成工单/申请复测/批量忽略）。

#### R5.5 内容安全监测

1. 系统 SHALL 提供敏感内容列表（类型：涉黄/涉赌/涉毒/涉政，命中词句，AI 置信度）。
2. 系统 SHALL 提供敏感信息泄漏列表（类型：身份证/手机号/邮箱/AccessKey，命中位置）。
3. 系统 SHALL 提供页面篡改告警（篡改维度：标题/图片/正文，变更前后对比）与截图缩略图展示。

#### R5.6 暗链与木马监测

1. 系统 SHALL 提供暗链列表（暗链 URL/类型：隐藏外链/友情链接/SEO 黑帽）。
2. 系统 SHALL 提供木马列表（特征：iframe 嵌套/JS 混淆/Shellcode，沙箱分析报告）。
3. 系统 SHALL 提供双 UA 对比功能（切换 UA 查看页面差异）。

#### R5.7 Webshell 与钓鱼检测

1. 系统 SHALL 提供 Webshell 列表（检测路径/特征码/文件内容片段）。
2. 系统 SHALL 提供钓鱼列表（仿冒目标/域名相似度/证书异常）。

#### R5.8 可用性与网络监测

1. 系统 SHALL 提供 12 小时可用性点阵图（绿/红竖线，支持 HTTP/DNS/PING 三维度切换）。
2. 系统 SHALL 提供 24 小时响应时序折线图与 DNS 劫持/污染记录。
3. 系统 SHALL 提供端口服务监测（开放端口列表/服务指纹/新增端口告警/高危服务暴露）。

#### R5.9 安全情报中心

1. 系统 SHALL 提供情报列表（CVE/CNVD 编号/标题/严重程度/影响范围）。
2. 系统 SHALL 自动关联本组织资产技术栈，标记"受影响资产数"。
3. 系统 SHALL 提供情报订阅配置（数据源开关）。

#### R5.10 任务调度与策略

1. 系统 SHALL 提供策略模板（10 大引擎开关/扫描并发/超时/速率限制）。
2. 系统 SHALL 提供 Cron 定时计划（绑定资产分组 + 策略模板 + 时间窗口）。
3. 系统 SHALL 提供任务队列监控（排队/处理中/已完成数量，Worker 分配状态）与断点续扫状态展示。

#### R5.11 报告中心

1. 系统 SHALL 提供报告模板（执行摘要/漏洞详情/内容安全/可用性统计/整改建议）。
2. 系统 SHALL 支持定时报告（Cron 生成周报/月报）。
3. 系统 SHALL 支持报告导出（PDF 含水印/Excel 漏洞清单/按资产与时间范围导出截图合集）。

#### R5.12 团队管理（仅 org_admin）

1. 系统 SHALL 提供成员列表（头像/角色 Tag/状态/最后登录时间）。
2. 系统 SHALL 支持邀请成员（邮箱/手机号 + 角色选择）、移除/禁用成员/修改角色。

#### R5.13 系统设置（仅 org_admin）

1. 系统 SHALL 提供 Worker 节点管理（心跳/负载/版本/Bootstrap Token）。
2. 系统 SHALL 提供通知渠道配置（钉钉/企微/飞书 Webhook + 邮件 SMTP + 测试发送）。
3. 系统 SHALL 提供规则库管理（POC 列表/敏感词库/木马特征库/版本号）。
4. 系统 SHALL 提供审计日志（操作人/时间/类型/前后值），审计日志 SHALL 禁止修改与删除。
5. 系统 SHALL 提供 API Token 管理（细粒度权限/有效期）。

#### R5.14 平台管理（仅 super_admin）

1. 系统 SHALL 提供组织列表（名称/套餐/资产数/Worker 数/到期时间/状态）。
2. 系统 SHALL 支持创建/编辑/禁用组织。
3. 系统 SHALL 提供平台统计（总组织/总资产/总扫描次数/总事件数）与平台 Worker 总览。

#### R5.15 API 开放集成

1. 系统 SHALL 全量开放 REST API（Swagger 文档）。
2. 系统 SHALL 提供 API Token 认证（独立于 JWT，支持细粒度权限）。
3. 系统 SHALL 提供 Webhook 事件推送（事件发生时主动 POST 到客户配置的 URL）。

#### R5.16 部署模式

1. 系统 SHALL 支持私有化部署（单二进制一键安装，零外部依赖）。
2. 系统 SHALL 支持 SaaS 化部署（Docker/K8s 编排，Master 水平扩展读写分离，Worker 弹性伸缩）。
3. 系统 SHALL 提供部署验证流程：Docker/K8s/单二进制三种方式启动后执行 `/api/health` 探活，并通过建资产→下发任务→证据展示全链路验收。
