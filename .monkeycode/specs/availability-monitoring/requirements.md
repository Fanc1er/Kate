# Requirements Document — 可用性监测模块

## Introduction

CInsight 平台当前仅有可用性监测「半成品」：后端存在 `AvailabilityEngine`（HTTP 探针）、`AvailabilityPoint` 时序表（已建表但从未写入）、`WorkerNode` 工作节点表、`ScanWhitelist` 白名单表，但缺少时序数据采集、独立查询 API 与前端展示页面。本模块将可用性监测补全为完整产品能力，配套落地《企业级安全监测后台 — 交互逻辑规范》中与可用性监测相关的交互。

范围决策（已与用户确认）：
1. 新增独立轻量可用性探针调度，每 N 分钟对启用可用性监测的资产做 HTTP 探针，只写 `AvailabilityPoint` 时序，不跑完整安全扫描。
2. 分阶段交付：先产出完整需求与设计，实施按里程碑（M1/M2/M3）推进。
3. 工作节点拓扑展示 Worker 节点状态（主节点 + 各 worker 的在线/离线/负载/心跳）。

## Glossary

- **资产（Asset）**：纳入监测的站点/URL，模型 `Asset`，含 `url`、`name`、`group_name`、`importance`。
- **可用性探测（Probe）**：对单一资产发起一次 HTTP GET，记录状态码、响应耗时、采样时间。
- **轻量探针调度（Probe Scheduler）**：独立于扫描任务的高频调度，仅执行可用性探测并写时序点。
- **时序点（AvailabilityPoint）**：单次探测的记录（资产、引擎、状态码、响应耗时、采样时间）。
- **可用性状态（Availability Status）**：由时序点聚合的站点健康状态，取值为正常/异常/未知。
- **不可用（Unreachable）**：连续 N 次（默认 3）探测失败判定的状态。
- **响应速度异常（Slow Response）**：响应耗时超过阈值（默认 3000ms）判定的状态。
- **工作节点（Worker Node）**：执行探测/扫描的节点，模型 `WorkerNode`，含名称、IP、状态、负载、心跳时间。
- **白名单（Whitelist）**：domain/ip/cidr 规则，命中后不再对可用性异常生成告警。
- **重新探测（Reprobe）**：用户手动立即触发一次可用性探测。

## Requirements

### R1: 轻量可用性探针调度

**User Story:** AS 运维人员，I want 系统按固定频率自动探测站点可用性，so that 我能实时掌握站点健康状态。

#### Acceptance Criteria

1. The system SHALL 按可配置频率（默认 5 分钟）对启用可用性监测的资产发起 HTTP GET 探测。
2. WHEN 探测完成, the system SHALL 将状态码、响应耗时、采样时间写入一条 `AvailabilityPoint`。
3. WHEN 探测调度运行, the system SHALL 仅执行轻量 HTTP 探测，而不创建完整安全扫描任务。
4. IF 探测频率未配置, the system SHALL 使用默认频率 5 分钟。
5. WHILE 授权状态无效, the system SHALL 停止发起新的可用性探测。

### R2: 可用性异常判定

**User Story:** AS 运维人员，I want 系统自动判定站点不可用与响应速度异常，so that 我能及时处置故障。

#### Acceptance Criteria

1. WHEN 资产连续 3 次探测失败, the system SHALL 判定该资产不可用。
2. IF 响应耗时超过 3000ms, the system SHALL 判定该资产响应速度异常。
3. WHEN 资产判定为不可用, the system SHALL 生成一条 `unreachable` 类型 finding。
4. IF 资产返回 HTTP 状态码 ≥ 400, the system SHALL 生成一条 `http_error` 类型 finding。
5. IF 资产响应耗时超过阈值, the system SHALL 生成一条 `slow_response` 类型 finding。
6. WHEN 资产恢复可用, the system SHALL 停止生成不可用告警。

### R3: 站点可用性状态聚合

**User Story:** AS 安全运营人员，I want 查看所有站点的最新可用性状态，so that 我能快速定位故障站点。

#### Acceptance Criteria

1. The system SHALL 为每个资产聚合最新可用性状态（正常/异常/未知）。
2. The system SHALL 为每个资产提供最新状态码、响应耗时、最后探测时间。
3. WHEN 资产从未被探测, the system SHALL 将该资产可用性状态标记为「未知」。
4. WHEN 资产最近一次探测状态码为 2xx/3xx 且耗时未超阈值, the system SHALL 标记为「正常」。
5. IF 资产最近一次探测失败或状态码 ≥ 400 或耗时超阈值, the system SHALL 标记为「异常」。

### R4: 站点可用性列表 API

**User Story:** AS 安全运营人员，I want 通过筛选条件查询站点可用性列表，so that 我能高效定位问题站点。

#### Acceptance Criteria

1. The system SHALL 提供站点可用性列表查询接口，返回资产列表及其最新可用性状态。
2. WHEN 用户按状态码筛选, the system SHALL 返回状态码匹配的站点。
3. WHEN 用户按可用性状态筛选, the system SHALL 返回对应状态的站点。
4. WHEN 用户输入关键词, the system SHALL 匹配站点名或 URL。
5. The system SHALL 支持分页（页码 + 每页条数）。
6. WHEN 多组筛选同时生效, the system SHALL 组内按 OR 组合、组间按 AND 组合。

### R5: 单站点可用性时序查询

**User Story:** AS 安全运营人员，I want 查看单站点 24 小时可用性时序，so that 我能分析故障发生与恢复时间。

#### Acceptance Criteria

1. The system SHALL 提供单资产 24 小时可用性时序查询接口。
2. WHEN 查询, the system SHALL 返回按采样时间升序的时序点数组，每个点含状态码、响应耗时、采样时间。
3. WHEN 指定时间范围, the system SHALL 返回该范围内的时序点。
4. WHEN 资产在该时间段无时序点, the system SHALL 返回空数组。

### R6: 重新探测

**User Story:** AS 安全运营人员，I want 手动触发站点重新探测，so that 我能即时刷新站点可用性状态。

#### Acceptance Criteria

1. WHEN 用户对单站点触发重新探测, the system SHALL 立即对该站点发起一次可用性探测并更新其状态。
2. WHEN 用户对多个站点触发批量重新探测, the system SHALL 对所选站点逐一发起探测。
3. WHEN 重新探测完成, the system SHALL 通知前端刷新该站点状态。
4. IF 站点不在白名单内, the system SHALL 允许重新探测。

### R7: 白名单操作

**User Story:** AS 安全运营人员，I want 将站点加入白名单，so that 我不再接收该站点的可用性误报告警。

#### Acceptance Criteria

1. WHEN 用户将站点加入白名单, the system SHALL 记录 domain/ip/cidr 白名单规则。
2. WHILE 站点命中白名单规则, the system SHALL 对该站点的可用性异常不生成告警。
3. WHEN 用户将站点移出白名单, the system SHALL 恢复对该站点的可用性告警。
4. The system SHALL 支持查询白名单规则列表。

### R8: 前端可用性监测页面

**User Story:** AS 安全运营人员，I want 在独立页面查看和管理站点可用性，so that 我能集中监测所有站点健康状态。

#### Acceptance Criteria

1. The system SHALL 在侧边栏提供「可用性监测」菜单入口。
2. WHEN 用户进入页面, the system SHALL 展示站点可用性列表（站点名、URL、可用性状态、状态码、响应耗时、最后探测时间）。
3. The system SHALL 提供中栏筛选面板，含状态码分组、可用性状态分组、关键词搜索框。
4. WHEN 用户勾选筛选, the system SHALL 实时刷新右侧站点表格，无需确认按钮。
5. The system SHALL 在表格内为每个站点展示 24 小时响应耗时 Sparkline。
6. WHEN 用户点击站点行, the system SHALL 打开该站点详情（Sparkline 放大 + 重新探测 + 加入白名单操作）。

### R9: 前端工作节点拓扑

**User Story:** AS 管理员，I want 查看工作节点拓扑，so that 我能掌握探测节点的运行状态。

#### Acceptance Criteria

1. The system SHALL 展示工作节点拓扑，含主节点与各 worker 节点。
2. WHEN 节点在线, the system SHALL 以绿色标识该节点。
3. IF 节点心跳超时, the system SHALL 以灰色标识该节点为离线。
4. WHEN 用户悬停节点, the system SHALL 显示节点名称、状态、负载、最后心跳时间。
5. WHEN 用户点击节点, the system SHALL 跳转该节点监控详情。

### R10: 全局搜索

**User Story:** AS 安全运营人员，I want 通过全局搜索快速定位站点/资产/发现，so that 我能快速跳转目标对象。

#### Acceptance Criteria

1. WHEN 用户按下 Cmd/Ctrl+K, the system SHALL 唤起全局搜索框并自动聚焦。
2. WHEN 用户输入关键词, the system SHALL 实时匹配站点名、URL 与发现标题。
3. WHEN 用户按 Esc, the system SHALL 关闭全局搜索框。
4. WHEN 用户选中搜索结果, the system SHALL 跳转对应详情页。

### R11: 交互规范落地（通用）

**User Story:** AS 用户，I want 界面交互符合统一规范，so that 我能获得一致且高效的操作体验。

#### Acceptance Criteria

1. WHEN 表格数据加载, the system SHALL 显示骨架屏而非整页空白。
2. WHEN 接口报错, the system SHALL 以顶部 Toast 通知用户。
3. WHEN 用户执行删除/移除等危险操作, the system SHALL 弹出二次确认弹窗。
4. WHEN 用户点击表头, the system SHALL 切换排序（升序→降序→取消）。
5. WHEN 用户切换每页条数, the system SHALL 支持 10/20/50/100 档位并回到第 1 页。
6. WHEN 用户通过键盘 Tab 聚焦交互元素, the system SHALL 显示绿色焦点框。
7. WHEN 数字统计值变化, the system SHALL 以数字滚动动画呈现。
8. WHEN 用户操作系统偏好为减少动效, the system SHALL 关闭非必要动效。

## 里程碑划分

| 里程碑 | 需求范围 | 说明 |
|--------|----------|------|
| M1 采集与基础列表 | R1、R2、R3、R4、R5、R8(1-5) | 探针调度 + 时序采集 + 列表/时序 API + 基础页面（列表/筛选/Sparkline） |
| M2 操作增强 | R6、R7、R8(6) | 重新探测、白名单、详情操作、批量操作 |
| M3 拓扑与全局 | R9、R10、R11 | 工作节点拓扑、全局搜索、通用交互规范落地 |
