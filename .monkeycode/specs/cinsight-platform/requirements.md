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
6. WHEN 用户退出登录，系统 SHALL 提供 `POST /api/v1/auth/logout`：注销当前会话（refresh token 入 jti 黑名单），前端 SHALL 清除本地 JWT 并跳转登录页。
7. WHEN 用户请求当前用户信息，系统 SHALL 提供 `GET /api/v1/auth/me` 返回当前用户资料（用户名/邮箱/角色/当前组织/头像/权限码集），前端据此渲染顶部导航与按钮权限；me 接口 SHALL 只依赖 JWT，不需要 `X-Org-Id`。
8. WHEN 用户 `status` 为 `disabled`（禁用用户）提交登录，系统 SHALL 拒绝登录返回 403 `USER_DISABLED`，不签发 JWT；WHEN 用户所在组织被禁用，系统 SHALL 同样拒绝登录并返回 403 `ORG_DISABLED`。
9. WHEN 用户持有已签发 JWT 后账户被禁用或所在组织被禁用，系统 SHALL 在后续受保护 API 校验时拒绝（401/403）并使该 JWT 失效。

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
8. 系统 SHALL 维护操作级权限矩阵（见 design「RBAC 权限矩阵与权限码」）：`viewer` 对所有写操作返回 403；`org_admin` 拥有本组织全部配置/管理写权限；`engineer` 拥有资产/任务/事件/告警/漏洞/工单/证据的写权限，策略/计划/成员/Worker/通知/规则库等配置类写权限仅 `org_admin`；前端菜单、路由、按钮三级共用同一权限码数据源。

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
1. 系统 SHALL 维护 `organizations` 表（id, name, logo_path, plan, max_assets, max_workers, max_members, expire_at, status）。
2. 系统 SHALL 维护 `users` 表（id, username, password(bcrypt), email, phone, avatar_url, status, last_login_at, is_super_admin）。
3. 系统 SHALL 维护 `user_orgs` 关联表（user_id, org_id, role, status, joined_at），并禁止 `super_admin` 加入 `user_orgs`。
4. 所有业务查询 SHALL 强制附带 `org_id` 过滤条件，未携带则拒绝。
5. 系统 SHALL 维护独立 `vulnerabilities` 表与 `alerts` 表：`vulnerabilities` 记录漏洞实体（cve_id/severity/status/evidence_id），`alerts` 记录告警实体（alert_type/severity/status/resolved_at），均由引擎发现记录触发生成。

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
4. WHEN 执行内容安全监测，引擎 SHALL 调用多端 UA 综合评估器（R2.12）：以随机 PC UA、随机移动端 UA、无头浏览器移动视口模拟（移动 UA + 手机宽度视口）三探针抓取比对，WHEN 任一探针命中敏感词/敏感信息或与基线偏差超阈，系统 SHALL 将对应维度计入综合评分并输出端级结论（哪一端异常、异常类型）。

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
4. WHEN 执行 HTTP 可用性监测，引擎 SHALL 调用多端 UA 综合评估器（R2.12）多探针探测，WHEN 端间可用性不一致（如某端返回错误、重定向降级、移动端独有拦截页），系统 SHALL 标记端差异化异常并计入综合评分。

#### R2.8 端口服务监测引擎

1. WHEN 执行端口扫描，引擎 SHALL 以 TCP SYN 方式扫描 Top 常见端口（22/21/3389/3306/6379 等）。
2. IF SYN 扫描需要特权但 Worker 以非 root 运行，引擎 SHALL 自动降级为 TCP Connect 全连接扫描（无特权要求），降级结果 SHALL 标记 `scan_mode: connect` 供审计追溯。
3. 引擎 SHALL 通过 Banner 抓取识别服务类型与版本。
4. WHEN 检测到新增开放端口或高危服务暴露（如 Redis 6379 对外开放），系统 SHALL 生成告警。

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

#### R2.12 多端 UA 综合评估器（MultiUA）

**User Story:** AS 安全工程师，I want 同一页面经多端视角比对评分，SO THAT 能识别单端隐藏的敏感内容、端差异化宕机与条件性篡改。

**Acceptance Criteria:**
1. 系统 SHALL 提供多端 UA 综合评估器组件，执行时以三探针抓取同一目标：随机 PC 端 UA（桌面视口 1440x900）、随机移动端 UA（移动视口）、无头浏览器移动 UA + 手机宽度视口（如 375x812）模拟，各探针 SHALL 独立记录状态码、响应时间、标题、图片 Hash、正文 DOM 结构、外链集合与敏感词命中。
2. 评估器 SHALL 对四个维度加权评分并输出 0~100 综合分：可用性一致性（各端状态/延迟/是否端差异化宕机或拦截）、内容命中（任一探针命中敏感词/敏感信息/AI 判定）、篡改偏差（与基线标题/图片 Hash/正文 DOM 对比）、条件性内容（某端独有外链/脚本/iframe，UA 条件加载）。
3. 评估器 SHALL 按综合分输出结论分级：0-30 正常 / 30-60 可疑（建议复核）/ 60-85 高危 / 85-100 严重，并输出端级明细（哪一端异常、异常类型、对应维度得分），随 finding 的 Extra 字段回传。
4. WHEN 任一探针抓取失败或超时（>30s），该探针 SHALL 标记 `probe_failed` 不计入权重，其余探针正常评估，失败探针单列展示。
5. 各探针抓取的 HTML 快照与截图 SHALL 随证据链 gzip 落盘 + SHA-256 入库，供前端多端对比展示与取证。
6. 评估器 SHALL 受策略模板开关与并发/超时约束，Worker 低资源环境可仅启用 PC + 移动端两探针（跳过无头浏览器模拟）。

### 3. 全量证据链与防篡改

#### R3.1 证据内容采集

**User Story:** AS 安全工程师，I want 每个漏洞/告警有完整证据链，SO THAT 可追溯可取证。

**Acceptance Criteria:**
1. 每个漏洞/告警 SHALL 关联证据链，包含请求信息（HTTP 方法、URL、Headers、Body）、响应信息（状态码、Headers、Body）、HTML 源码快照、代码位置片段（精确到行号）与截图取证。
2. 引擎 SHALL 输出漏洞触发点的关键代码片段与行号（如 `<script>alert(1)</script>` 在第 152 行）。
3. 每条 finding SHALL 携带置信度字段 `confidence`（0~1，引擎判定可信度）与代码行号 `line_no`（触发点行号，无代码定位时为空）。
4. Worker SHALL 在采集 Req/Resp 证据时按 HAR 1.2 规范组装生成 HAR 文件（entries 含请求/响应头、Body、时间戳、大小、MIME 类型），随证据链 gzip 落盘并入库，供前端下载与工具导入。

#### R3.2 证据存储与校验

1. 大体积证据数据（HTML 源码、完整响应体）SHALL 经 gzip 压缩落盘至 `/data/evidence/{date}/`，数据库仅存元数据、文件路径与 SHA-256 签名。
2. 证据文件 SHALL 通过 MD5 去重，SHA-256 签名入库。
3. WHEN 前端展示证据，后端 SHALL 强制校验文件 Hash，IF 校验不一致，UI SHALL 标红提示"证据已被破坏"。
4. Worker→Master 证据传输 SHALL 走专用接口 `POST /api/v1/worker/evidence`：Worker 先将证据 gzip 分片上传（单片 ≤ 8MB，含片序号与总片数），Master 收齐后合并落盘并复算 SHA-256 与入库；传输中断 SHALL 支持断点续传（按已收分片跳过），全部完成后在结果回传中引用 evidence_id。小体积证据（<1MB）SHALL 允许随结果 JSON 内联回传，无需走分片。

#### R3.2b Worker 页面截图取证（无头浏览器组件）

1. Worker SHALL 内嵌无头浏览器截图组件（chromedp 或独立 chromium 进程），负责页面渲染截图取证。
2. 引擎产出 finding 需截图时 SHALL 调用无头浏览器渲染（viewport 1440x900，DOMContentLoaded + 2s 等待）并输出 PNG，随证据链 gzip 落盘 + SHA-256 入库。
3. 浏览器不可用或渲染超时（>30s）时 SHALL 降级为不截图，仅保留 Req/Resp/HTML 证据并标记 `screenshot: skipped`，不阻断任务。
4. 同一 URL 同一任务内截图结果 SHALL 缓存（BadgerDB），避免重复渲染；截图并发受池约束，Worker 低资源环境可禁用截图（策略开关）。

#### R3.3 前端证据展示

1. 系统 SHALL 提供通用证据读取接口 `GET /api/v1/evidence/{id}`，返回证据元数据（Req/Resp/HTML/截图）并经 Hash 校验。
2. 前端 SHALL 提供全屏抽屉，上方左右分屏展示 HTTP Req/Resp。
3. 下方 SHALL 展示 HTML 源码（代码高亮 + 行号标红定位）。
4. 前端 SHALL 提供截图取证标签页展示渲染截图，并提供下载完整 HTML 快照 / HAR 文件按钮。
5. HTML 源码属不可信外部内容，前端 SHALL 禁止直接 `v-html` 注入；渲染前 SHALL 经 DOMPurify 白名单净化（剥离 script/style/iframe/on* 事件属性等），净化后高亮展示，防止存储型 XSS 经证据链回放。

#### R3.4 截图上传接口

**User Story:** AS 取证人员，I want 上传页面渲染截图，SO THAT 证据链包含可视化取证材料。

**Acceptance Criteria:**
1. 系统 SHALL 提供 `POST /api/v1/evidence/screenshots` 上传接口，接受 Base64 或文件流。
2. 上传接口 SHALL 经鉴权（JWT/API Token）并校验文件类型与大小，上传后 SHALL 经 gzip 落盘并计算 SHA-256 入库。
3. 截图 SHALL 关联目标资产或证据记录，可在证据抽屉截图标签页展示。
4. 上传文件 SHALL 校验 MIME 类型（仅允许 png/jpeg/webp）与大小上限（默认 10MB），超限或类型不符 SHALL 拒绝并返回 4001。
5. 文件名 SHALL 防路径穿越：禁止 `..`、`/`、`\` 等字符，落盘文件名 SHALL 由服务端生成 UUID，忽略客户端文件名。

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
5. SQLite SHALL 配置单写连接池（`MaxOpenConns=1`）与连接参数（`ConnMaxLifetime` 30m、`ConnMaxIdleTime` 10m、`busy_timeout` 5s），写冲突由事务重试兜底；只读副本连接池独立配置。
6. 删除业务（单条/批量/级联清理）SHALL 同步删除对应 Bleve 索引文档，保持全文索引与 SQLite 一致；SHALL 提供 `index rebuild` 全量重建命令用于故障后对账。

#### R4.3 高可用容灾

1. Worker 断网时 SHALL 将结果缓存至本地 Outbox，网络恢复后批量回传。
2. Worker SHALL 以 `local:crawled:{task_id}` 记录已爬取 URL，重启后跳过已扫页面实现断点续扫。
3. Master 启动时 SHALL 将超时（30min）未回传的 `processing` 任务重置为 `pending`。
4. Master SQLite SHALL 通过 Litestream 实时流式备份，RPO ≤ 5s，备份保留 30 天。
5. Master SHALL 支持只读副本水平扩展（查询路由到副本，写入收敛单写通道），单 Master 支撑 10 万资产 / 100 Worker。
6. 结果回传 SHALL 携带幂等键（`result_id`，Worker 端生成 UUID），Master SHALL 按幂等键去重，重复回传 SHALL 忽略并返回已接收。
7. 任务失败 SHALL 自动重试，重试上限 3 次，超过上限 SHALL 置为 `failed` 并生成告警事件。
8. 任务 SHALL 配置任务级超时上限（默认 60min，可经策略模板覆盖）：超过上限未完成 SHALL 中止并置为 `failed`，防止单个任务无限占驻 Worker；Worker 侧执行超时同样终止并在结果中标记 `task_timeout`。
9. Master 收到 SIGTERM SHALL 优雅停机：停止接收新任务、排空在途 HTTP 请求与任务对账、完成 Outbox 落盘后退出，退出超时 30s。

#### R4.4 深度安全对抗

1. 资产 URL 入库前 SHALL 强制标准化（协议/默认端口/路径），BadgerDB 维护 MD5 防重。
2. 扫描 SHALL 执行授权拦截：白名单校验、内网 IP 禁止，IF 违规 3 次，系统 SHALL 自动熔断 Worker。授权白名单 SHALL 由 org_admin 管理（`GET/PUT /api/v1/scan-whitelist`：允许扫描的目标域名/IP/网段 + 全局内网 IP 段黑名单），Worker 每次发起请求前 SHALL 以 Hash 同步白名单并本地校验。
3. 系统 SHALL 预留 Proxy（HTTP/SOCKS5）配置与低速隐蔽模式（并发=1、伪造 UA/Referer）反封禁设计。
4. 目标请求 SHALL 经 `gobreaker` 熔断，IF 连续失败 5 次，系统 SHALL 熔断该目标。
5. 身份证、手机号等敏感数据 SHALL 在入库前、API 返回前、报告生成时三时机自动脱敏。
6. Master SHALL 通过 `fsnotify` 监听规则文件热加载，Worker SHALL 定期拉取 Hash 同步规则库。

#### R4.5 可观测性（企业级）

1. 系统 SHALL 暴露 `GET /metrics`（Prometheus 格式），采集 RED + USE + 业务指标（任务队列/事件数/Outbox 存量/Worker 心跳）。
2. 系统 SHALL 提供 `GET /healthz`（存活）与 `GET /readyz`（就绪：SQLite/Badger/证据目录/Litestream），供 K8s 探针使用。
3. 系统 SHALL 以 OpenTelemetry 输出 `trace_id` 贯穿 Master→Worker→外部调用，采样率 10%（错误路径全采样）。
4. 系统 SHALL 满足 SLO：API 可用性 ≥ 99.9%，p99 延迟 < 500ms，任务成功率 ≥ 99%。
5. 系统 SHALL 输出结构化日志（JSON），含 `ts/level/org_id/user_id/trace_id/path/latency_ms/status`，请求中间件逐请求记录。
6. 日志 SHALL 对敏感字段（密码/Token/身份证/手机号/Headers）自动脱敏后输出，禁止明文落日志。
7. 系统 SHALL 提供备份恢复与演练脚本（backup.sh/restore.sh/drill.sh），恢复演练 SHALL 验证数据完整性与可启动性。

#### R4.6 安全加固（企业级）

1. 系统 SHALL 经网关终结 TLS，响应头含 CSP / X-Frame-Options / X-Content-Type-Options / HSTS。
2. 系统 SHALL 提供 API 通用限流（每用户/IP 100 req/min，超限返回 HTTP 429 + `Retry-After` 头）与登录接口独立限流（5 次/min/IP，连续失败 5 次 SHALL 锁定账户 15 分钟，防暴力破解）。
3. 系统 SHALL 执行密码策略：≥ 12 位含大小写/数字/特殊字符，90 天轮换，禁止复用最近 5 次，首次登录强制改密。
4. 系统 SHALL 提供密码重置流程：`POST /api/v1/auth/forgot-password`（邮件验证码）+ `POST /api/v1/auth/reset-password`，重置后 SHALL 失效旧 token 并强制重新登录。登录态改密 SHALL 走 `POST /api/v1/auth/change-password`（校验旧密码 + 符合密码策略），改密后 SHALL 立即失效全部 refresh token 并强制重新登录。验证码邮件与成员邀请邮件 SHALL 由系统级 SMTP 发送（`CINSIGHT_SMTP_*` 配置），独立于组织通知渠道，保证登录前/未入组场景可用；验证码 SHALL 5 分钟内有效、一次性使用。
5. 系统 SHALL 预留 MFA（TOTP）二次认证开关。
6. 系统 SHALL 将 Secrets（JWT_SECRET/Webhook secret/Bootstrap Token）经环境/K8s Secret 注入，支持轮换，严禁写入代码与日志。
7. 系统 SHALL 将依赖漏洞扫描（govulncheck）与容器镜像扫描（Trivy）纳入 CI 门禁。
8. WebSocket 握手 SHALL 校验 JWT 与 org_id，订阅通道 SHALL 绑定连接所属 org，禁止越权订阅/接收他组织事件。
9. 数据写入 SHALL 支持乐观锁：核心表（assets/scan_policies/alerts/tickets）SHALL 含 `version` 字段，更新时携带 `If-Match` 或请求体 version，版本不匹配 SHALL 返回 409。
10. 系统 SHALL 采用 access token（15min）+ refresh token（7d）双 token 机制，提供 `POST /api/v1/auth/refresh`；登出、改密、换组织、密码重置后 SHALL 立即失效全部 refresh token（服务端 jti 黑名单）。

#### R4.7 数据治理与合规（企业级）

1. 系统 SHALL 提供数据保留与归档：事件/漏洞/告警热数据 180 天后冷归档，审计日志保留 ≥ 365 天。归档后的历史数据 SHALL 保留只读查询能力（列表/详情可查，写操作不可执行），查询接口复用现有列表/详情端点并带 `archived=true` 参数路由到归档库。
2. 证据文件 SHALL 按保留期（默认 365 天）定时清理：删除孤儿文件、回收磁盘空间，删除前 SHALL 校验无证据记录引用。
3. 系统 SHALL 支持账户注销与个人信息删除/匿名化，满足 PIPL/GDPR 数据生命周期要求。
4. 系统 SHALL 提供数据可携权导出（GDPR）：用户可导出本人全部个人数据（账户信息、操作记录、审计相关条目），格式 JSON/CSV 下载，默认保留 72 小时，导出动作 SHALL 写入审计日志。
5. 系统 SHALL 按等保 2.0 对齐身份鉴别、访问控制、安全审计、数据完整性与保密性要求。

### 5. 功能模块需求

#### R5.1 仪表盘（全部角色可见）

1. 系统 SHALL 展示统计卡片（资产总数/可用率/高危漏洞/待处理事件）。
2. 系统 SHALL 展示 7 天趋势图（漏洞发现/事件趋势）、资产风险 Top10 排行、实时事件 WebSocket 滚动。
3. 系统 SHALL 展示引擎覆盖率雷达图（10 大引擎检测覆盖率）。
4. 系统 SHALL 提供仪表盘聚合后端接口 `GET /api/v1/dashboard/stats`（统计卡片）与 `GET /api/v1/dashboard/trends`（7 天趋势）、`GET /api/v1/dashboard/top-risks`（风险 Top10）、`GET /api/v1/dashboard/engine-coverage`（引擎覆盖率），各卡片与图表独立拉取、互不阻塞。

#### R5.2 资产管理

1. 系统 SHALL 提供资产后端 API：CRUD（`GET/POST /api/v1/assets`、`GET/PUT/DELETE /api/v1/assets/:id`）。
2. 系统 SHALL 提供资产画像接口 `GET /api/v1/assets/:id/profile`（技术栈指纹/ICP 备案/子域名/SSL 证书倒计时/端口服务快照）。
3. 系统 SHALL 提供资产变更追踪接口 `GET /api/v1/assets/:id/history`（标题/技术栈/状态码/端口变动历史）。
4. 系统 SHALL 在资产入库前执行 URL 归一化，并通过 BadgerDB MD5 防重。
5. 系统 SHALL 提供微信公众号资产接口完整 CRUD（`GET/POST /api/v1/wechat-assets`、`GET/PUT/DELETE /api/v1/wechat-assets/:id`），字段包含公众号名、微信号、头像、粉丝数、简介、认证状态与文章数。
6. 系统 SHALL 提供批量操作接口：`POST /api/v1/assets/batch-scan`（批量加入扫描）、`POST /api/v1/assets/batch-delete`（批量删除）、`POST /api/v1/assets/batch-group`（批量改分组）、`POST /api/v1/assets/batch-import`（URL 列表/CSV 批量导入，含模板下载与逐行校验报告）。
7. 系统 SHALL 提供导入模板下载 `GET /api/v1/assets/import-template`（返回 URL/CSV 模板文件）与当前筛选结果 CSV 导出 `GET /api/v1/assets/export?filter[..]`（导出当前筛选条件下的资产字段，受 org_id 隔离约束）。
8. 系统 SHALL 提供资产列表前端（虚拟滚动/模糊搜索/按重要程度/分组/状态筛选）、多选批量操作栏与资产画像/变更追踪抽屉展示。
9. 系统 SHALL 在资产列表为空时展示空状态引导，并提供"立即添加/批量导入"主操作。

#### R5.3 安全事件中心

1. 系统 SHALL 提供事件列表（级别/来源资产/引擎类型/内容/状态流转：待处理→处理中→已关闭→已归档）。
2. 系统 SHALL 支持事件类型筛选：漏洞/内容违规/暗链挂马/木马/Webshell/钓鱼/篡改/可用性异常/端口暴露/敏感信息泄漏/信誉异常/情报预警。
3. 系统 SHALL 提供降噪规则配置完整 CRUD（`GET/POST /api/v1/noise-rules`、`PUT/DELETE /api/v1/noise-rules/:id`），规则类型包含白名单 IP/忽略特定类型/聚合时间窗/风暴抑制。
4. 系统 SHALL 提供批量状态流转接口 `POST /api/v1/events/batch`（批量确认/关闭/归档）。
5. 系统 SHALL 支持闭环处置流程：事件确认→工单派发→修复跟踪→复测验证→归档，并自动挂载应急响应 SOP。

#### R5.3b 独立告警中心

**User Story:** AS 安全工程师，I want 独立查看与处置告警，SO THAT 聚焦需优先响应的高危发现。

**Acceptance Criteria:**
1. 系统 SHALL 提供独立告警列表接口 `GET /api/v1/alerts`（按级别/类型/状态/来源资产筛选与分页）。
2. 系统 SHALL 提供告警处置接口 `PATCH /api/v1/alerts/:id`（确认/关闭/静默三种状态流转）。
3. 系统 SHALL 提供批量告警处置接口 `POST /api/v1/alerts/batch`（批量确认/关闭/静默，请求体 `{ids, action}`）。
4. 系统 SHALL 以独立 `alerts` 表存储告警（含 alert_type、severity、status、resolved_at），由发现记录触发生成。

#### R5.4 漏洞与风险管理

1. 系统 SHALL 提供漏洞列表（按等级/状态/引擎类型筛选），基于独立 `vulnerabilities` 表。
2. 系统 SHALL 提供漏洞证据接口 `GET /api/v1/vulnerabilities/{id}/evidence`，聚合漏洞关联的 Req/Resp/HTML/截图证据链。
3. 系统 SHALL 提供证据链抽屉（Req/Resp 分屏 + HTML 行号高亮 + 截图取证）。
4. 系统 SHALL 支持闭环处置（生成工单/申请复测/忽略），并提供批量接口 `POST /api/v1/vulnerabilities/batch-ticket`、`batch-retest`、`batch-ignore`。
5. 系统 SHALL 提供漏洞行内快捷操作（生成工单/申请复测/忽略）与筛选持久化。

#### R5.5 内容安全监测

1. 系统 SHALL 提供敏感内容列表（类型：涉黄/涉赌/涉毒/涉政，命中词句，AI 置信度）。
2. 系统 SHALL 提供敏感信息泄漏列表（类型：身份证/手机号/邮箱/AccessKey，命中位置）。
3. 系统 SHALL 提供页面篡改告警（篡改维度：标题/图片/正文，变更前后对比）与截图缩略图展示。
4. 系统 SHALL 提供 AI 内容分类服务适配层：endpoint/model/api_key 经环境注入（`CINSIGHT_AI_ENDPOINT`/`CINSIGHT_AI_MODEL`/`CINSIGHT_AI_API_KEY`，由管理员配置），失败回退内置敏感词正则引擎，判定结果 SHALL 标记来源 `ai` 或 `regex`。
5. 系统 SHALL 提供多端 UA 综合评估结果展示（R2.12）：展示 PC/移动/移动视口模拟三探针的对比明细（各端状态码、响应时间、敏感词命中、DOM 指纹差异、独有外链）与综合评分及结论分级，并标注端级异常定位。

#### R5.6 暗链与木马监测

1. 系统 SHALL 提供暗链列表（暗链 URL/类型：隐藏外链/友情链接/SEO 黑帽）。
2. 系统 SHALL 提供木马列表（特征：iframe 嵌套/JS 混淆/Shellcode，沙箱分析报告）。
3. 系统 SHALL 提供双 UA 对比功能（切换 UA 查看页面差异）。
4. 系统 SHALL 提供双 UA 对比触发接口 `POST /api/v1/assets/:id/dual-ua`：后端安排 Worker 分别以正常 UA 与蜘蛛 UA 抓取目标页面，返回两次抓取的标题/外链/脚本/正文差异列表，供前端对比展示。

#### R5.7 Webshell 与钓鱼检测

1. 系统 SHALL 提供 Webshell 列表（检测路径/特征码/文件内容片段）。
2. 系统 SHALL 提供钓鱼列表（仿冒目标/域名相似度/证书异常）。

#### R5.8 可用性与网络监测

1. 系统 SHALL 提供 12 小时可用性点阵图（绿/红竖线，支持 HTTP/DNS/PING 三维度切换）。
2. 系统 SHALL 提供 24 小时响应时序折线图与 DNS 劫持/污染记录。
3. 系统 SHALL 提供端口服务监测（开放端口列表/服务指纹/新增端口告警/高危服务暴露）。
4. 系统 SHALL 提供时序查询接口 `GET /api/v1/assets/:id/availability?engine=http&hours=12`（点阵图）与 `GET /api/v1/assets/:id/response-time?hours=24`（响应折线），数据来自时序降级表（availability_points），按 org_id 隔离。
5. 系统 SHALL 展示多端 UA 可用性评估结果（R2.12）：PC/移动/移动视口模拟三探针的状态码与响应时间对比、端差异化异常（如仅移动端拦截/降级）与综合评分结论，并支持下载各端抓取快照。

#### R5.9 安全情报中心

1. 系统 SHALL 提供情报列表（CVE/CNVD 编号/标题/严重程度/影响范围）。
2. 系统 SHALL 自动关联本组织资产技术栈，标记"受影响资产数"。
3. 系统 SHALL 提供情报订阅配置（数据源开关）。
4. 系统 SHALL 提供情报查询接口 `GET /api/v1/intel`（列表，按来源/严重程度/关键字筛选分页）与 `GET /api/v1/intel/:id`（详情，含受影响资产列表），并标记"受影响资产数"由引擎自动关联本组织资产技术栈计算。

#### R5.10 任务调度与策略

1. 系统 SHALL 提供策略模板（10 大引擎开关/扫描并发/超时/速率限制），完整 CRUD（`GET/POST /api/v1/policies`、`PUT/DELETE /api/v1/policies/:id`）、复制模板 `POST /api/v1/policies/:id/copy` 与批量删除 `POST /api/v1/policies/batch-delete`。
2. 系统 SHALL 提供 Cron 定时计划（绑定资产分组 + 策略模板 + 时间窗口），完整 CRUD（`GET/POST /api/v1/scan-plans`、`PUT/DELETE /api/v1/scan-plans/:id`）、启停开关 `PATCH /api/v1/scan-plans/:id/status` 与批量启停 `POST /api/v1/scan-plans/batch-toggle`。Cron 表达式 SHALL 绑定明确时区（默认 `Asia/Shanghai`，可经 `CINSIGHT_TIMEZONE` 覆盖），服务端按该时区计算执行时间，前端创建/编辑时展示所选时区。
3. 系统 SHALL 提供任务列表、任务详情 `GET /api/v1/tasks/:id`（状态/进度/执行日志/结果统计/Worker 分配）、停止 `POST /api/v1/tasks/:id/stop`、失败重跑 `POST /api/v1/tasks/:id/rerun`、删除历史任务 `DELETE /api/v1/tasks/:id`、批量停止 `POST /api/v1/tasks/batch-stop` 与批量失败重跑 `POST /api/v1/tasks/batch-rerun`。
4. 系统 SHALL 提供任务队列监控（排队/处理中/已完成数量，Worker 分配状态）与断点续扫状态展示。

#### R5.11 报告中心

1. 系统 SHALL 提供报告模板（执行摘要/漏洞详情/内容安全/可用性统计/整改建议），完整 CRUD（`GET/POST /api/v1/report-templates`、`PUT/DELETE /api/v1/report-templates/:id`）。
2. 系统 SHALL 支持定时报告（Cron 生成周报/月报），Cron 执行时区 SHALL 与扫描计划一致（默认 `Asia/Shanghai`，`CINSIGHT_TIMEZONE` 可覆盖）。
3. 系统 SHALL 支持报告导出（PDF 含水印/Excel 漏洞清单/按资产与时间范围导出截图合集），提供报告详情 `GET /api/v1/reports/:id`、删除 `DELETE /api/v1/reports/:id` 与异步生成进度通知。

#### R5.12 团队管理（仅 org_admin）

1. 系统 SHALL 提供成员列表（头像/角色 Tag/状态/最后登录时间）。
2. 系统 SHALL 支持邀请成员（邮箱/手机号 + 角色选择）、批量邀请 `POST /api/v1/members/batch-invite`、批量移除 `POST /api/v1/members/batch-remove`、修改角色、禁用/启用成员、移除成员 `DELETE /api/v1/members/:id`（移除二次确认）。

#### R5.13 系统设置（仅 org_admin）

1. 系统 SHALL 提供 Worker 节点管理（心跳/负载/版本/Bootstrap Token），心跳通过 `POST /api/v1/worker/heartbeat` 上报，支持移除离线节点 `DELETE /api/v1/worker/nodes/:id`。
2. Worker 注册 SHALL 走握手流程：首次注册用一次性 Bootstrap Token 调用 `POST /api/v1/worker/register` 换取长期凭证（client_id + client_secret，服务端存 hash），后续心跳/拉取/回传 SHALL 用长期凭证鉴权；凭证支持撤销/重发，泄露可吊销。
3. Bootstrap Token 与长期凭证 SHALL 加密/哈希存储（Bootstrap Token 存 SHA-256 且一次性领取即失效，client_secret 存 bcrypt），库中禁止明文；Token 仅在创建时一次性返回，刷新即作废旧值。
4. 受邀成员 SHALL 首次登录激活：邀请状态 `invited`，首次登录后激活为 `active` 并强制设密码/改密；邀请链接默认 7 天过期。
5. 系统 SHALL 提供通知渠道配置完整 CRUD（`GET/POST /api/v1/notify-channels`、`PUT/DELETE /api/v1/notify-channels/:id`，钉钉/企微/飞书 Webhook + 邮件 SMTP 多渠道），并按 id 测试发送 `POST /api/v1/notify-channels/:id/test`。
6. 系统 SHALL 提供通知路由规则配置 `GET/PUT /api/v1/notify-routes`：按事件类型与严重级别（如 critical/high 走钉钉 + 邮件、medium 走企微、低危静默）映射到具体渠道，未命中路由的告警 SHALL 按默认渠道发送；渠道启用开关与风暴抑制（R4.2-4）在路由层生效。
7. 通知渠道的密钥/令牌类字段（Webhook Secret、SMTP 密码等）SHALL 加密存储（AES-256-GCM，主密钥经环境变量注入），接口返回时 SHALL 脱敏（仅掩码显示），编辑回显 SHALL 提供"留空则保持原值"。
8. Worker 注册 SHALL 受组织 Worker 配额约束：已注册 Worker 数达到该组织 `max_workers` 时，注册握手 SHALL 返回 4291 `WORKER_QUOTA_EXCEEDED` 拒绝注册；移除节点释放配额。
9. 系统 SHALL 提供规则库管理（POC 列表/敏感词库/木马特征库/版本号），规则项支持增删改查（`GET/POST /api/v1/rules/items`、`PUT/DELETE /api/v1/rules/items/:id`），并支持批量导入导出（`GET/POST /api/v1/rules/import`、`GET /api/v1/rules/export`）。
10. 系统 SHALL 提供情报订阅配置独立接口 `GET/PUT /api/v1/intel-subscriptions`（CVE/CNVD/CNNVD 数据源开关）。
11. 系统 SHALL 提供审计日志（操作人/时间/类型/前后值），支持按操作人、操作类型、资源类型、时间范围筛选（`GET /api/v1/audit-logs?operator=&action=&resource_type=&start=&end=`）与分页；审计日志 SHALL 记录客户端 IP 与 User-Agent（服务端请求中间件捕获，不依赖前端上报），并禁止修改与删除。
12. 系统 SHALL 提供 API Token 管理（细粒度权限/有效期），支持撤销与临时停用/恢复 `PATCH /api/v1/api-tokens/:id/status`。

#### R5.14 平台管理（仅 super_admin）

1. 系统 SHALL 提供组织列表（名称/套餐/资产数/Worker 数/到期时间/状态）。
2. 系统 SHALL 支持创建/编辑/禁用/启用组织（`POST /api/v1/orgs/:id/disable` 与 `POST /api/v1/orgs/:id/enable`），提供组织详情 `GET /api/v1/orgs/:id` 与删除组织 `DELETE /api/v1/orgs/:id`（删除需输入组织名二次确认并级联清理数据）；启用后 SHALL 恢复该组织 cron 计划与写操作。
3. 系统 SHALL 提供平台统计（总组织/总资产/总扫描次数/总事件数）与平台 Worker 总览。
4. 系统 SHALL 按组织套餐执行配额校验（核心商业逻辑）：创建资产时已用资产数达到 `max_assets` 返回 4290 `ASSET_QUOTA_EXCEEDED`；邀请成员达到 `max_members` 返回 4292 `MEMBER_QUOTA_EXCEEDED`、注册 Worker 达到 `max_workers` 返回 4291 `WORKER_QUOTA_EXCEEDED`；批量操作逐条校验不中断，超限条目计入 failed 并返回原因。
5. WHEN 组织到期（`expire_at` 已过）或组织被禁用，系统 SHALL 停止该组织的定时计划、拒绝新建任务与资产变更，仅保留只读查询与证据下载。

#### R5.15 API 开放集成

1. 系统 SHALL 全量开放 REST API（Swagger 文档）。
2. 系统 SHALL 提供 API Token 认证（独立于 JWT，支持细粒度权限）。
3. 系统 SHALL 提供 Webhook 事件推送（事件发生时主动 POST 到客户配置的 URL）。推送 SHALL 带 HMAC-SHA256 签名（请求头 `X-Signature`，密钥经 R5.18 管理），推送失败 SHALL 自动重试 3 次（指数退避），重试仍失败 SHALL 记录推送状态落库并在 UI 标记送达失败。
4. Swagger 文档（`/swagger/*`）SHALL 受开关控制：生产环境默认关闭，仅当 `CINSIGHT_SWAGGER_ENABLED=true` 时暴露，防止生产环境信息泄露。

#### R5.16 部署模式与 CI/CD

1. 系统 SHALL 支持私有化部署（单二进制一键安装，零外部依赖）。
2. 系统 SHALL 支持 SaaS 化部署（Docker/K8s 编排，Master 水平扩展读写分离，Worker 弹性伸缩）。
3. 系统 SHALL 提供部署验证流程：Docker/K8s/单二进制三种方式启动后执行 `/api/health` 探活，并通过建资产→下发任务→证据展示全链路验收。
4. 系统 SHALL 提供 CI/CD 流水线：lint（golangci-lint + go vet）→ 单测（go test -race -cover）→ 构建 → 镜像安全扫描 → 部署 dev/staging/prod。
5. 系统 SHALL 采用 SemVer 版本管理与 CHANGELOG，核心包行覆盖率 ≥ 80% 作为质量门禁。
6. 系统 SHALL 提供 E2E 测试（Playwright 覆盖登录→资产→任务→证据→报告关键路径）与 k6 压测验证 SLO。
7. 系统 SHALL 提供集成测试，覆盖认证→资产→任务下发→引擎结果回传→事件→闭环全链路。

#### R5.17 前端工程化与体验

1. 系统 SHALL 提供路由守卫：`beforeEach` 校验 token 有效性、组织选择态与 role 权限。
2. 系统 SHALL 提供按钮级权限指令 `v-permission`，菜单/路由/按钮三级权限 SHALL 一致（同一 RBAC 权限表驱动，禁止某级绕过）。
3. 系统 SHALL 提供全局错误处理：axios 拦截器统一错误提示 + 异常兜底页 + 全局错误边界组件。
4. 系统 SHALL 在页面加载提供骨架屏占位，请求失败 SHALL 显示 Toast 提示并支持重试。
5. 系统 SHALL 在 WebSocket 断线时显示断线提示条，重连成功 SHALL 自动恢复并清除提示；WebSocket SHALL 提供心跳保活（每 30s ping，服务端 60s 读超时关闭死连接，连续 3 次无 pong 触发重连）。
6. 系统 SHALL 预留 i18n（中/英）与可访问性支持，路由懒加载 + 组件分包保证首屏 < 3s。
7. 系统 SHALL 提供前端组件测试（vitest）：覆盖登录表单校验、路由守卫权限、权限菜单渲染。
8. 所有列表页 SHALL 提供空状态引导（插画 + 文案 + 主操作按钮），危险操作（删除/移除/撤销）SHALL 二次确认弹窗。
9. 列表多选后 SHALL 显示批量操作栏（已选 N 项/全选/跨页记忆），批量结果 SHALL Toast 汇总"成功 M / 失败 K"并可展开失败明细。
10. 资产 URL、API Token、Webhook Secret、Bootstrap Token SHALL 提供一键复制；资产列表 SHALL 支持 URL/CSV 批量导入（模板下载 + 逐行校验报告）与当前筛选结果 CSV 导出。
11. 导航栏事件/告警 SHALL 显示未读数角标（WebSocket 实时更新）；任务行 SHALL 支持点击展开详情（进度/日志/结果统计）。
12. 列表筛选条件 SHALL 持久化（localStorage）并支持 URL 参数分享；报告生成 SHALL 异步化并展示进度条 + 完成通知。
13. 系统 SHALL 提供统一批量操作规范：`POST /api/v1/{resource}/batch` 携带 `{ids}`（上限 500），返回 `{success, failed}`，逐条失败不中断，批量操作幂等。

#### R5.18 Webhook 配置管理

1. 系统 SHALL 提供 Webhook 完整 CRUD（`GET/POST /api/v1/webhooks`、`PUT/DELETE /api/v1/webhooks/:id`）。
2. 系统 SHALL 提供 Webhook 测试推送 `POST /api/v1/webhooks/:id/test`，测试结果 SHALL 反馈 HTTP 状态码与响应体。
3. 系统 SHALL 支持签名密钥管理：`POST /api/v1/webhooks/:id/secret` 重新生成 HMAC-SHA256 密钥，编辑时显示 Secret 一键复制与重新生成。

#### R5.19 前端 UX 基座

1. 系统 SHALL 提供全局 Toast 与 MessageBox 二次确认弹窗封装，错误提示/危险操作确认/成功反馈统一走封装组件。
2. 系统 SHALL 提供通用表格基座：多选批量操作栏（已选 N 项/全选/跨页记忆）、分页、排序、筛选重置、日期选择、空态、骨架屏、搜索防抖（300ms）。
3. 系统 SHALL 提供统一表单校验规则库（必填/URL/邮箱/手机号/密码强度）与新建/编辑共用抽屉组件，保存中 SHALL 禁用按钮防重复提交。
4. 系统 SHALL 提供图表基座：ECharts 统一配色、阈值配色（高危红/中危橙/低危黄/正常绿）、角色 Tag 颜色规范（super_admin 紫/org_admin 蓝/engineer 青/viewer 灰）。
5. 系统 SHALL 提供通用详情抽屉基座（Req/Resp 分屏 + HTML 行号高亮 + 截图 tab + 下载 + 时间线），资产画像/漏洞证据/事件详情 SHALL 复用。
6. 系统 SHALL 确保各业务页（资产/事件/告警/漏洞/任务/成员/策略/计划）批量操作与通用表格基座对齐。
