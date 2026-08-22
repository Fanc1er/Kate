# CInsight 离线授权与单租户化 — 技术设计文档

> Feature: `offline-license` · Updated: 2026-08-22

## 1. 概述

本设计将 CInsight 从「多租户 SaaS」改造为「单租户 + 离线授权」部署形态，包含两条主线：

1. **离线授权机制**：以机器特征（硬盘序列号、CPU ID、MAC、OS 版本 + 随机盐值）为锚点，客户导出授权码 → 厂商用离线工具签发加密授权文件 → 客户导入解锁。授权文件同时携带时间额度（有效期、延迟激活、续期提醒）与资源额度（资产数上限、Worker 数上限）。无有效授权时，后端仅暴露授权相关接口，前端仅渲染导入授权页。
2. **单租户化改造**：移除 `org_id` 数据隔离、`Organization`/`UserOrg` 表、`X-Org-Id` 中间件与四层角色，简化为 `admin`/`user` 两级，数据从零初始化。

两条主线在代码层面相互交织，实施顺序为「先授权门禁 → 再单租户化」，保证每一阶段都可编译可运行。

## 2. 授权文件机制

### 2.1 密码学方案

授权文件采用「**对称加密 + 非对称签名**」双层保护：

| 层 | 算法 | 用途 | 密钥归属 |
|----|------|------|----------|
| 加密 | AES-256-GCM | 加密授权载荷，防止明文读取 | 派生密钥，两端一致 |
| 签名 | RSA-2048 + SHA-256 | 防篡改、防伪造 | 私钥仅厂商持有，公钥内置后端 |

- AES 密钥由固定逻辑派生：`aesKey = SHA-256("cinsight-license-v1" + 固定盐)`，签发工具与后端使用相同派生逻辑，无需额外配置。
- RSA 公钥以 PEM 常量内置后端（`internal/master/license/keys` 包），支持环境变量 `CINSIGHT_LICENSE_PUBLIC_KEY` 覆盖。
- RSA 私钥仅存在于厂商签发的离线工具，永不进入后端或交付给客户。

### 2.2 授权文件格式

授权文件为文本（JSON envelope），扩展名 `.lic`：

```json
{
  "format": "cinsight-license",
  "version": 1,
  "cipher": "AES-256-GCM",
  "nonce": "<base64: 12 字节随机 nonce>",
  "payload": "<base64: AES-GCM 加密后的载荷>",
  "signature": "<base64: RSA-SHA256 对 payload 密文签名>"
}
```

解密后的载荷（`payload` 明文，JSON）：

```json
{
  "machine_hash": "<SHA-256(机器特征 + 盐值)>",
  "issued_at": "2026-08-22T00:00:00Z",
  "not_before": "2026-09-01T00:00:00Z",
  "not_after": "2027-08-22T00:00:00Z",
  "max_assets": 1000,
  "max_workers": 5,
  "customer": "可选客户标识",
  "features": {}
}
```

- `not_before`：延迟激活时间；当前时间早于该点时授权未生效。
- `not_after`：授权截止时间；当前时间晚于该点时授权过期。
- `max_assets` / `max_workers`：资源额度上限；`0` 表示不限制。
- `features` 为预留扩展字段，用于未来增加更多额度维度。

### 2.3 授权码与机器特征

机器特征采集（Linux，纯 sysfs / 标准库，无外部命令依赖）：

| 特征 | 来源 | 说明 |
|------|------|------|
| 硬盘序列号 | `/sys/block/*/device/serial` | 主特征，跨系统重装不变 |
| CPU ID | `/proc/cpuinfo` 的 `vendor_id` + `model name` + `stepping` + `physical id` | 主特征；Linux 通常不暴露 CPU 序列号，用上述字段组合 |
| MAC 地址 | `net.Interfaces()` 取首个非 loopback 网卡 | 辅助特征 |
| OS 版本 | `/etc/os-release` 的 `PRETTY_NAME` + 内核版本（`/proc/sys/kernel/osrelease`） | 辅助特征 |

生成流程：

1. 采集上述 4 个特征原始值。
2. 盐值：首次采集时用 `crypto/rand` 生成 16 字节随机值，持久化到 `{DataDir}/machine.salt`（权限 0600）；后续复用该文件，保证机器码稳定。
3. 组合：`硬盘序列号|CPU ID|MAC|OS 版本|hex(盐值)`。
4. 机器哈希 = `SHA-256(组合串)`，授权码 = 机器哈希的 hex 小写表示（可读、便于复制）。

- 盐值持久化在客户机器本地，厂商与授权码均不包含盐值，防止厂商或第三方通过预计算哈希表逆向匹配硬件特征。
- 授权码仅暴露机器特征摘要，不泄露任何凭据；采集来源随授权码一并返回用于前端提示。

### 2.4 授权签发工具

新增独立 CLI：`tools/license-issuer/`（`package main`，不进入后端依赖）。

```bash
license-issuer \
  -key ./private.pem \
  -machine "机器码hex" \
  -not-before 2026-09-01 \
  -not-after 2027-08-22 \
  -max-assets 1000 \
  -max-workers 5 \
  -customer "客户名称" \
  -out license.lic
```

- 复用 `internal/master/license` 包中的载荷结构、AES 派生与签名逻辑，保证签发与验证一致。
- 密钥生成：`openssl genrsa -out private.pem 2048` + `openssl rsa -in private.pem -pubout -out public.pem`。
- 仓库在 `tools/license-issuer/` 下提供开发用密钥对（私钥用于本地签发调试，公钥内容同步内置到后端 keys 包）。

## 3. 后端改造

### 3.1 数据模型（`internal/master/models/models.go`）

- 删除模型：`Organization`、`UserOrg`。
- 删除所有业务模型的 `OrgID` 字段及其索引（`Asset`、`WechatAsset`、`ScanPolicy`、`ScanTask`、`Finding`、`SensitiveInfoHit`、`RuleDefinition`、`Vulnerability`、`Alert`、`Event`、`Ticket`、`Evidence`、`EvidenceFile`、`ScanPlan`、`AuditLog`、`APIToken`、`NotifyChannel`、`NotifyRoute`、`NoiseRule`、`ScanWhitelist`、`WorkerNode`、`IntelSubscription`、`ReportTemplate`、`Report`、`Webhook`、`AvailabilityPoint`、`TrendPoint`、`EscalationRule`、`WatchShift`、`DailyWarReport`、`Scenario`、`SOPTemplate`、`ContentBaseline`、`ExternalLinkBaseline`）。
- `User` 变更：`IsSuperAdmin bool` → `Role string`（取值 `admin` / `user`）。
- 新增模型 `License`（用于审计记录导入事件，非授权真相源）：

```go
type License struct {
    ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
    MachineHash string    `gorm:"size:64;not null" json:"machine_hash"`
    NotBefore   time.Time `json:"not_before"`
    NotAfter    time.Time `json:"not_after"`
    MaxAssets   int       `json:"max_assets"`
    MaxWorkers  int       `json:"max_workers"`
    Customer    string    `gorm:"size:256" json:"customer"`
    ImportedAt  time.Time `json:"imported_at"`
}
```

- 角色常量：`RoleSuperAdmin/RoleOrgAdmin/RoleEngineer/RoleViewer` → `RoleAdmin/RoleUser`。

### 3.2 授权管理器（新增 `internal/master/license/`）

```
internal/master/license/
├── license.go      // Payload、Status、Manager（Load/Import/Check/MachineCode/Status/配额访问）
├── cipher.go       // AES-256-GCM 加解密、SHA-256 派生
├── sign.go         // RSA-SHA256 签名与验签
├── machine.go      // 机器特征采集（硬盘序列号/CPU ID/MAC/OS 版本）与盐值持久化
└── keys/
    └── keys.go     // 内置 RSA 公钥 PEM 常量 + 读取环境变量覆盖
```

核心接口：

```go
type Status string
const (
    StatusValid           Status = "valid"
    StatusMissing         Status = "missing"
    StatusInvalid         Status = "invalid"
    StatusNotYetActive    Status = "not_yet_active"
    StatusExpired         Status = "expired"
    StatusMachineMismatch Status = "machine_mismatch"
)

type Manager struct {
    mu        sync.RWMutex
    filePath  string          // {DataDir}/license.lic
    saltPath  string          // {DataDir}/machine.salt
    publicKey *rsa.PublicKey
    aesKey    []byte
    status    Status
    payload   *Payload
}

func (m *Manager) Load() error                 // 启动加载授权文件
func (m *Manager) Import(data []byte) Status   // 校验并写入 + 刷新内存状态
func (m *Manager) Check() Status               // 运行时实时校验，返回状态
func (m *Manager) Status() Status              // 只读状态
func (m *Manager) MachineCode() (code, source string, err error)
func (m *Manager) MaxAssets() int              // 授权资产上限（0=不限）
func (m *Manager) MaxWorkers() int             // 授权 Worker 上限（0=不限）
func (m *Manager) DaysRemaining() int          // 距 not_after 剩余天数，用于续期提醒
```

- `Check()` 逻辑：无文件 → `StatusMissing`；签名无效 → `StatusInvalid`；机器哈希不匹配 → `StatusMachineMismatch`；`now < not_before` → `StatusNotYetActive`；`now > not_after` → `StatusExpired`；否则 `StatusValid`。
- 授权文件为真相源，存于 `{DataDir}/license.lic`；`Manager` 用 `RWMutex` 保护内存状态，导入时原子刷新。
- 过期/未生效检查：每次请求经中间件实时比对内存缓存的 `not_before`/`not_after` 与当前时间，避免依赖定时器。

### 3.3 资源额度强制

- 资产上限：`service/asset.go` 创建资产前统计资产总数，`count >= Manager.MaxAssets()` 时返回 `CodeAssetQuota`（`MaxAssets() == 0` 时不限制）。
- Worker 上限：`service/worker.go` 注册/心跳接入新节点前统计活跃 Worker 数，`count >= Manager.MaxWorkers()` 时返回 `CodeWorkerQuota`（`MaxWorkers() == 0` 时不限制）。
- 配额校验通过 `Service` 注入 `*license.Manager` 实现，避免中间件层耦合。

### 3.4 License 中间件与路由（`middleware/security.go`、`routes/routes.go`）

`Security` 增加 `License *license.Manager` 字段。新增中间件：

```go
func (s *Security) LicenseRequired() gin.HandlerFunc {
    return func(c *gin.Context) {
        switch s.License.Check() {
        case license.StatusValid:
            c.Next()
        default:
            response.Fail(c, s.LicenseError())  // 按状态映射错误码
            c.Abort()
        }
    }
}
```

路由重组（`routes.go` 的 `Setup`）：

```
/api/health                                  → 无门禁（健康检查）
/api/v1/license/machine-code                 → 无门禁（GET 授权码）
/api/v1/license/status                       → 无门禁（GET 授权状态 + 续期提醒）
/api/v1/license/import                       → 无门禁（POST 导入授权文件）
/api/v1/*（其余全部）                         → LicenseRequired 门禁
  ├─ /auth/*（登录/刷新/登出/改密）
  ├─ /worker/*、/ws/events
  └─ authed（AuthRequired）
       ├─ /auth/me
       ├─ /admin/*（用户管理，RequireAdmin）
       └─ 业务路由（assets/tasks/policies/triage/reports/evidence/dashboard/members/worker-nodes）
```

- `LicenseRequired` 位于所有业务与认证接口之前，无授权时登录接口同样返回授权错误码。
- `GET /api/v1/license/status` 返回 `{ status, days_remaining, not_before, not_after, max_assets, max_workers }`，前端据此渲染锁定页或续期提醒。
- 新增 `registerLicense(api, d)` 注册三个授权接口。
- 后台调度同样受授权门禁约束：`CronScheduler` 注入 `*license.Manager`，`tick()` 开头校验 `Check() != StatusValid` 时直接返回，授权无效即停止定时触发新的扫描任务（满足「运行时授权校验」AC：授权无效停止调度）。

### 3.5 认证与角色简化

- `AuthRequired`：删除 `X-Org-Id` 逻辑，改为从 `User.Role` 注入 `role`，删除 `is_super_admin` 注入。
- 删除 `OrgRequired`、`RequireSuperAdmin`。
- 新增 `RequireAdmin`：校验 `role == admin`。
- `RequireRoles` 保留，用于两级角色白名单。
- `RequireWrite` 保留，改为 `RequireRoles(admin, user)`（两级角色均拥有业务读写权限）；`AdminOnly` 保留为 `RequireAdmin` 的兼容别名。

### 3.6 服务层 `org_id` 移除

- `service/auth.go`：`Login` 直接返回用户 + JWT，删除 `user_orgs` 查询与 `SelectOrg` 方法；`Me` 返回 `Role`。
- 其余 service：删除方法签名中的 `orgID` 参数，删除 SQL 中 `WHERE org_id = ?` 过滤与 `org_id` 赋值。
- `service/seed.go`：初始化默认 `admin` 账号（角色 `admin`），删除组织种子与配额逻辑。
- `pkg/rbac`：角色常量与权限映射更新为 `admin`/`user`。

### 3.7 错误码（`pkg/errs/errs.go`）

新增：

```go
CodeLicenseRequired        = 2400 // 缺少有效授权
CodeLicenseInvalid         = 2401 // 授权文件无效（签名/格式）
CodeLicenseExpired         = 2402 // 授权已过期
CodeLicenseMachineMismatch = 2403 // 机器不匹配
CodeLicenseNotYetActive    = 2404 // 授权未生效（延迟激活）
```

`HTTPStatus` 均映射 `403 Forbidden`。保留资源配额错误码 `CodeAssetQuota = 4290`、`CodeWorkerQuota = 4291`（HTTP 429），用于授权资源额度。删除/停用 `CodeOrgRequired`、`CodeOrgDisabled`、`CodeOrgLimitExceeded`、`CodeMemberQuota`。

### 3.8 配置（`pkg/config/config.go`）

新增：

```go
LicensePath      string // CINSIGHT_LICENSE_PATH，默认 {DataDir}/license.lic
LicensePublicKey string // CINSIGHT_LICENSE_PUBLIC_KEY，可选，覆盖内置公钥
```

## 4. 前端改造

### 4.1 路由守卫（`router/index.ts`）

改造 `beforeEach`，在任何登录/角色判断之前先校验授权状态：

```
若未拉取 license 状态 → 请求 GET /api/v1/license/status（缓存于 store）
若 license.status != "valid" 且 to.path != "/license" → 重定向 /license
若 license.status == "valid" 且 to.path == "/license" → 重定向 /login
```

- 新增 `views/license/LicenseView.vue`（导入授权页），加入 `STATIC_ROUTES`（`/license`）。
- 删除 `SelectOrgView.vue` 与 `/select-org` 路由。
- 删除 `requiresOrg` 元信息与 `getOrgId()` 相关判断。
- 角色判断改为 `admin`/`user`。

### 4.2 导入授权页（`views/license/LicenseView.vue`）

- 加载时请求 `/api/v1/license/machine-code`，展示授权码与「复制」按钮、机器特征来源。
- 提供授权文件上传控件，调用 `/api/v1/license/import` 上传 `.lic`。
- 导入成功后刷新授权状态并跳转 `/login`。
- 当 `days_remaining <= 30` 时展示续期提醒；当 `status == "not_yet_active"` 时展示生效时间。

### 4.3 状态管理与拦截器

- 新增 `stores/license.ts`：维护 `status`、`machineCode`、`daysRemaining`，提供 `fetchStatus`、`importLicense`。
- 新增 `api/license.ts`：`machineCode`、`status`、`import` 三个请求封装。
- `api/http.ts`：
  - 移除 `X-Org-Id` 请求头注入与 `getOrgId/setOrgId/clearOrgId`。
  - 响应拦截器识别授权错误码（2400–2404），清除 token 并跳转 `/license`。
- `stores/auth.ts`：移除 `organizations`、`orgId`、`selectOrg` 相关状态与方法。

### 4.4 角色与菜单

- `config/routes.ts`：`roles` 取值改为 `admin`/`user`；用户管理（`members`）、平台管理（`platform`）归入 `admin`；其余业务路由 `roles` 改为 `['admin','user']` 或留空。
- `MENU` 同步更新角色过滤。

## 5. 数据迁移与初始化

- 采用「清空重来」策略：单租户版本使用无 `org_id` 的新 schema，旧多租户数据库不迁移、不读取；旧多租户运行时数据（`cinsight.db`/`badger`/`evidence`）已清理删除。
- 首次启动 `seed` 创建默认 `admin` 账号（角色 `admin`），密码来自 `CINSIGHT_SUPER_ADMIN_PASS`（沿用现有环境变量，空则随机生成并打印）。
- `License` 表记录最近一次导入事件，用于审计，不参与运行时校验。
- 盐值文件 `{DataDir}/machine.salt` 首次生成后持久化，删除会导致机器码变化（视为机器更换）。

## 6. 测试策略

后端（`go test ./...`）：
- `license` 包：`MachineCode` 稳定性（盐值持久化后两次一致）；`Import` 覆盖有效、签名篡改、机器不匹配、未生效、过期五态；`Check` 状态机；`Load` 无文件/坏文件。
- `middleware`：`LicenseRequired` 门禁放行与拦截。
- `service/asset`：资产配额（达到上限拒绝新增）。
- `service/worker`：Worker 配额（达到上限拒绝接入）。
- `service/auth`：登录不再依赖组织、`Me` 返回角色。
- `errs`：新增错误码映射。

前端：`npm run typecheck` + `npm run build`。

集成验证：
1. 无授权启动 → `curl /api/v1/license/status` 返回 `missing`，`curl /api/v1/auth/login` 返回 2400。
2. 签发工具生成授权文件 → 导入 → 状态 `valid` → 登录成功。
3. 篡改授权文件 → 导入失败（2401）。
4. 过期授权 → 导入失败（2402）且运行中锁定。
5. 未生效授权（`not_before` 在未来）→ 状态 `not_yet_active`（2404）。
6. 不同机器特征 → 导入失败（2403）。
7. 资产/Worker 超配额 → 拒绝新增（4290 / 4291）。

## 7. 安全考量

- RSA 私钥永不进入后端与交付物，仅存在于厂商签发工具；公钥可内置或环境变量覆盖。
- 授权文件即使被解密读取，无法伪造（RSA 签名保护完整性）。
- 机器特征与随机盐值仅以哈希形式进入授权文件，避免明文序列号落盘；盐值持久化于客户本地，不随授权码外泄。
- `LicenseRequired` 覆盖认证与业务全部接口，杜绝绕过；仅 `/api/health` 与 `/api/v1/license/*` 无门禁。
- 授权码不包含任何密钥、口令、盐值或可反向推导的敏感信息。
