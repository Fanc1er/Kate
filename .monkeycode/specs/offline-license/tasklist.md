# 需求实施计划

- [x] 1. 后端授权核心包（internal/master/license/）
  - [x] 1.1 定义授权载荷与状态（license.go）
    - 定义 Payload（machine_hash/issued_at/not_before/not_after/max_assets/max_workers/customer/features）
    - 定义 Status 枚举（valid/missing/invalid/not_yet_active/expired/machine_mismatch）
    - 定义 Manager（Load/Import/Check/Status/MachineCode/MaxAssets/MaxWorkers/DaysRemaining）
  - [x] 1.2 实现 AES-256-GCM 加解密（cipher.go）
    - SHA-256 派生 AES 密钥，载荷加解密
  - [x] 1.3 实现 RSA-SHA256 签名与验签（sign.go）
    - 对载荷密文签名/验签
  - [x] 1.4 实现机器特征采集与盐值持久化（machine.go）
    - 采集硬盘序列号/CPU ID/MAC/OS 版本
    - 盐值生成与持久化（crypto/rand 16 字节，{DataDir}/machine.salt）
    - 机器码 = hex(SHA-256(特征|盐值))
  - [x] 1.5 内置 RSA 公钥（keys/keys.go）
    - PEM 常量 + CINSIGHT_LICENSE_PUBLIC_KEY 覆盖
  - [x] 1.6 为 license 包编写单元测试
    - MachineCode 稳定性、Import 五态、Check 状态机、Load 无文件/坏文件

- [x] 2. 授权签发工具（tools/license-issuer/）
  - [x] 2.1 实现 CLI 签发逻辑（main.go）
    - 参数 key/machine/not-before/not-after/max-assets/max-workers/customer/out
    - 复用 license 包生成 .lic 文件
  - [x] 2.2 提供开发用 RSA 密钥对（private.pem + public.pem）
    - 公钥内容同步内置到 keys 包

- [x] 3. 检查点 - 确保授权包与签发工具可编译、可互操作
  - 签发工具生成 .lic → 后端 Manager 导入验证通过

- [x] 4. 后端授权门禁
  - [x] 4.1 新增授权错误码（pkg/errs/errs.go）
    - CodeLicenseRequired/Invalid/Expired/MachineMismatch/NotYetActive
    - 保留 CodeAssetQuota/CodeWorkerQuota，删除租户相关错误码
  - [x] 4.2 新增授权配置（pkg/config/config.go）
    - LicensePath/LicensePublicKey
  - [x] 4.3 实现 LicenseRequired 中间件（middleware/security.go）
    - 按 Status 映射错误码
  - [x] 4.4 注册授权路由（routes/routes.go + license_routes.go）
    - GET /license/machine-code、GET /license/status、POST /license/import
  - [x] 4.5 路由重组，LicenseRequired 门禁覆盖业务与认证接口
    - auth/worker/ws/authed/业务路由挂到 licensed 组
  - [x] 4.6 后台调度授权门禁（停止调度）
    - CronScheduler 注入 License，tick 校验授权无效即停止定时触发扫描任务

- [x] 5. 后端单租户化
  - [x] 5.1 数据模型改造（models/models.go）
    - 删除 Organization/UserOrg，删除全部业务模型 OrgID 字段及索引
    - User.IsSuperAdmin → User.Role，新增 License 模型
    - 角色常量改为 RoleAdmin/RoleUser
  - [x] 5.2 中间件角色简化（middleware/security.go）
    - 删除 OrgRequired/RequireSuperAdmin
    - AuthRequired 注入 role，新增 RequireAdmin/RequireRoles
    - RequireWrite 保留为 RequireRoles(admin,user)，AdminOnly 保留为 RequireAdmin 别名
  - [x] 5.3 认证服务改造（service/auth.go）
    - Login 删除 user_orgs 查询与 SelectOrg，Me 返回 Role
  - [x] 5.4 初始化服务改造（service/seed.go）
    - 默认 admin 账号（角色 admin），删除组织种子
  - [x] 5.5 业务服务 org_id 移除
    - asset/task/policy/triage/evidence/dashboard/member/report/worker 删除 orgID 参数与 SQL 过滤
  - [x] 5.6 资源配额强制（service/asset.go、service/worker.go）
    - 资产数、Worker 数上限校验
  - [x] 5.7 权限包更新（pkg/rbac）
    - 角色与权限映射改为 admin/user

- [x] 6. 检查点 - 确保后端 go vet/build/test 全绿
  - 修复因单租户化产生的编译错误与测试失败

- [x] 7. 前端改造
  - [x] 7.1 新增授权 API（api/license.ts）
    - machineCode/status/import
  - [x] 7.2 新增授权状态 store（stores/license.ts）
    - status/machineCode/daysRemaining + fetchStatus/importLicense
  - [x] 7.3 新增导入授权页（views/license/LicenseView.vue）
    - 授权码展示与复制、上传导入、续期提醒/未生效提示
  - [x] 7.4 路由守卫改造（router/index.ts）
    - 优先校验 license 状态，非 valid 强制 /license
    - 删除 select-org、requiresOrg、getOrgId
  - [x] 7.5 http 拦截器改造（api/http.ts）
    - 移除 X-Org-Id，识别 2400-2404 跳 /license
  - [x] 7.6 auth store 改造（stores/auth.ts）
    - 移除 organizations/orgId/selectOrg
  - [x] 7.7 路由配置与菜单角色改造（config/routes.ts）
    - 角色改 admin/user，用户管理（members）/平台管理（platform）归 admin

- [x] 8. 检查点 - 前端 typecheck/build 通过，后端全量回归
  - 集成验证：无授权/导入/篡改/过期/未生效/机器不匹配/配额
