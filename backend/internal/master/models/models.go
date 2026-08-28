// Package models 定义 Master SQLite 全量 GORM 模型（对应 design「核心表」）。
package models

import (
	"time"
)

// 角色枚举
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// 通用状态
const (
	StatusActive    = "active"
	StatusDisabled  = "disabled"
	StatusInvited   = "invited"
	StatusOnline    = "online"
	StatusOffline   = "offline"
	StatusDeleted   = "deleted"
	StatusPending   = "pending"
	StatusProcessing= "processing"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// User 用户。
type User struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Password     string     `gorm:"size:128;not null" json:"-"`
	Email        string     `gorm:"size:128" json:"email"`
	Phone        string     `gorm:"size:32" json:"phone"`
	AvatarURL    string     `gorm:"size:512" json:"avatar_url"`
	Status       string     `gorm:"size:32;default:active" json:"status"`
	Role         string     `gorm:"size:32;default:user" json:"role"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Asset 资产。
type Asset struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	URL        string    `gorm:"size:1024;not null" json:"url"`
	Name       string    `gorm:"size:256" json:"name"`
	GroupName  string    `gorm:"size:128;index:idx_assets_org_group" json:"group_name"`
	Importance string    `gorm:"size:16;default:medium;index:idx_assets_org_importance" json:"importance"`
	TechStack  string    `gorm:"type:text" json:"tech_stack"`
	ICP        string    `gorm:"size:64" json:"icp"`
	SSLExpire  string    `gorm:"size:32" json:"ssl_expire"`
	Status     string    `gorm:"size:32;default:active" json:"status"`
	Remark     string    `gorm:"size:1024" json:"remark"`
	SourceType string    `gorm:"size:32;default:manual" json:"source_type"`
	Version    int       `gorm:"default:1" json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// WechatAsset 微信公众号资产。
type WechatAsset struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name         string    `gorm:"size:256" json:"name"`
	WechatID     string    `gorm:"size:128" json:"wechat_id"`
	AvatarURL    string    `gorm:"size:512" json:"avatar_url"`
	FansCount    int       `json:"fans_count"`
	Intro        string    `gorm:"size:1024" json:"intro"`
	VerifyStatus string    `gorm:"size:64" json:"verify_status"`
	ArticleCount int       `json:"article_count"`
	Status       string    `gorm:"size:32;default:active" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ScanPolicy 策略模板。
type ScanPolicy struct {
	ID              int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name            string `gorm:"size:128;not null" json:"name"`
	Scenario        string `gorm:"size:32;default:daily" json:"scenario"`
	EngineSwitches  string `gorm:"type:text" json:"engine_switches"`
	Concurrency     int    `gorm:"default:4" json:"concurrency"`
	Timeout         int    `gorm:"default:60" json:"timeout"`
	RateLimit       int    `gorm:"default:10" json:"rate_limit"`
	ScanDepth       int    `gorm:"default:2" json:"scan_depth"`
	ConcurrencyLimit int   `gorm:"default:4" json:"concurrency_limit"`
	AllowStatic     bool   `gorm:"default:false" json:"allow_static"`
	SameOrigin      bool   `gorm:"default:true" json:"same_origin"`
	CrawlSubpages   bool   `gorm:"default:true" json:"crawl_subpages"`
	Version         int    `gorm:"default:1" json:"version"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ScanTask 扫描任务。
type ScanTask struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	PolicyID     int64     `json:"policy_id"`
	PlanID       int64     `json:"plan_id"`
	AssetID      int64     `gorm:"index:idx_tasks_org_created" json:"asset_id"`
	TaskScope    string    `gorm:"size:16;default:root" json:"task_scope"`
	Status       string    `gorm:"size:32;default:pending;index:idx_tasks_org_status" json:"status"`
	Progress     int       `gorm:"default:0" json:"progress"`
	RetryCount   int        `gorm:"default:0" json:"retry_count"`
	RetryAt      *time.Time `json:"retry_at"`
	OutboxState  string     `gorm:"size:64" json:"outbox_state"`
	StoppedByUser bool     `gorm:"default:false" json:"stopped_by_user"`
	WorkerID     string    `gorm:"size:128" json:"worker_id"`
	Message      string    `gorm:"size:1024" json:"message"`
	ProgressMeta string    `gorm:"type:text" json:"progress_meta"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time `gorm:"index:idx_tasks_org_created" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Finding 检测发现（引擎统一输出）。
type Finding struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      int64     `gorm:"index:idx_findings_org_engine" json:"task_id"`
	AssetID     int64     `json:"asset_id"`
	ResultID    string    `gorm:"size:64;index:idx_findings_result" json:"result_id"`
	EngineName  string    `gorm:"size:64;index:idx_findings_org_engine" json:"engine_name"`
	Type        string    `gorm:"size:64" json:"type"`
	Severity    string    `gorm:"size:16;index:idx_findings_org_severity" json:"severity"`
	RiskLevel   string    `gorm:"size:16;index:idx_findings_org_severity" json:"risk_level"`
	RiskScore   int       `gorm:"default:0" json:"risk_score"`
	Suggestion  string    `gorm:"size:512" json:"suggestion"`
	Title       string    `gorm:"size:512" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	URL         string    `gorm:"size:1024" json:"url"`
	LineNo      int       `json:"line_no"`
	Confidence  float64   `gorm:"default:0" json:"confidence"`
	EvidenceIDs string    `gorm:"type:text" json:"evidence_ids"`
	Status      string    `gorm:"size:32;default:open;index:idx_findings_org_status" json:"status"`
	Extra       string    `gorm:"type:text" json:"extra"`
	CreatedAt   time.Time `json:"created_at"`
}

// SensitiveInfoHit 敏感信息命中明细。
type SensitiveInfoHit struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TaskID      int64     `json:"task_id"`
	RuleID      int64     `json:"rule_id"`
	Group       string    `gorm:"size:64" json:"group"`
	Name        string    `gorm:"size:128" json:"name"`
	MatchedText string    `gorm:"type:text" json:"matched_text"`
	Scope       string    `gorm:"size:32" json:"scope"`
	URL         string    `gorm:"size:1024" json:"url"`
	Depth       int       `json:"depth"`
	CreatedAt   time.Time `json:"created_at"`
}

// RuleDefinition 规则项（敏感词/POC/木马特征/关键词/白名单）。
type RuleDefinition struct {
	ID       int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Kind     string `gorm:"size:32;not null" json:"kind"` // sensitive/poc/keyword/trojan/content_whitelist/domain_whitelist
	Group    string `gorm:"size:64" json:"group"`
	Name     string `gorm:"size:256" json:"name"`
	Loaded   bool   `gorm:"default:true" json:"loaded"`
	FRegex   string `gorm:"type:text" json:"f_regex"`
	SRegex   string `gorm:"type:text" json:"s_regex"`
	Format   string `gorm:"size:256" json:"format"`
	Color    string `gorm:"size:32" json:"color"`
	Scope    string `gorm:"size:64" json:"scope"`
	Engine   string `gorm:"size:16;default:regexp" json:"engine"`
	Sensitive bool  `gorm:"default:false" json:"sensitive"`
	Version  int    `gorm:"default:1" json:"version"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Vulnerability 漏洞实体。
type Vulnerability struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	FindingID   int64     `json:"finding_id"`
	AssetID     int64     `gorm:"index:idx_vuln_org_asset" json:"asset_id"`
	CVEID       string    `gorm:"size:64;index:idx_vuln_org_cve" json:"cve_id"`
	EngineName  string    `gorm:"size:64" json:"engine_name"`
	Severity    string    `gorm:"size:16;index:idx_vuln_org_severity" json:"severity"`
	Status      string    `gorm:"size:32;default:open;index:idx_vuln_org_status" json:"status"`
	Title       string    `gorm:"size:512" json:"title"`
	Description string    `gorm:"type:text" json:"description"`
	EvidenceIDs string    `gorm:"type:text" json:"evidence_ids"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ClosedAt    *time.Time `json:"closed_at"`
}

// Alert 告警实体。
type Alert struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID    int64      `gorm:"index:idx_alerts_org_asset" json:"asset_id"`
	FindingID  int64      `json:"finding_id"`
	AlertType  string     `gorm:"size:64;index:idx_alerts_org_type" json:"alert_type"`
	Severity   string     `gorm:"size:16;index:idx_alerts_org_severity" json:"severity"`
	Title      string     `gorm:"size:512" json:"title"`
	Content    string     `gorm:"type:text" json:"content"`
	Status     string     `gorm:"size:32;default:open;index:idx_alerts_org_status" json:"status"`
	Extra      string     `gorm:"type:text" json:"extra"`
	Version    int        `gorm:"default:1" json:"version"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at"`
}

// Event 安全事件。
type Event struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID     int64     `json:"asset_id"`
	FindingIDs  string    `gorm:"type:text" json:"finding_ids"`
	EngineName  string    `gorm:"size:64" json:"engine_name"`
	EventType   string    `gorm:"size:64;index:idx_events_org_type" json:"event_type"`
	Title       string    `gorm:"size:512" json:"title"`
	Severity    string    `gorm:"size:16;index:idx_events_org_severity" json:"severity"`
	URL         string    `gorm:"size:1024" json:"url"`
	Content     string    `gorm:"type:text" json:"content"`
	EvidenceIDs string    `gorm:"type:text" json:"evidence_ids"`
	Status      string    `gorm:"size:32;default:pending;index:idx_events_org_status" json:"status"`
	SOPAttached string    `gorm:"size:512" json:"sop_attached"`
	CreatedAt   time.Time `json:"created_at"`
}

// Ticket 工单。
type Ticket struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID    int64     `json:"event_id"`
	VulnID     int64     `json:"vuln_id"`
	Assignee   string    `gorm:"size:128" json:"assignee"`
	Status     string    `gorm:"size:32;default:open" json:"status"`
	DueAt      *time.Time `json:"due_at"`
	Notes      string    `gorm:"type:text" json:"notes"`
	Version    int       `gorm:"default:1" json:"version"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Evidence 证据元数据（大文件 gzip 落盘，库内仅存元数据与签名）。
type Evidence struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	MD5       string    `gorm:"size:64;index:idx_evidence_org_md5" json:"md5"`
	SHA256    string    `gorm:"size:64;not null" json:"sha256"`
	FilePath  string    `gorm:"size:1024" json:"file_path"`
	MimeType  string    `gorm:"size:128" json:"mime_type"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// EvidenceFile 证据链子文件（html/har/screenshot/req/resp）。
type EvidenceFile struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EvidenceID int64     `gorm:"index" json:"evidence_id"`
	Kind       string    `gorm:"size:32" json:"kind"`
	UploadID   string    `gorm:"size:128" json:"upload_id"`
	PartIndex  int       `json:"part_index"`
	PartTotal  int       `json:"part_total"`
	FilePath   string    `gorm:"size:1024" json:"file_path"`
	MD5        string    `gorm:"size:64" json:"md5"`
	SHA256     string    `gorm:"size:64" json:"sha256"`
	Size       int64     `json:"size"`
	MimeType   string    `gorm:"size:128" json:"mime_type"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// ScanPlan Cron 定时计划。
type ScanPlan struct {
	ID             int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name           string    `gorm:"size:128" json:"name"`
	PolicyID       int64     `json:"policy_id"`
	AssetGroupName string    `gorm:"size:128" json:"asset_group_name"`
	CronExpr       string    `gorm:"size:128" json:"cron_expr"`
	SubpageCronExpr string   `gorm:"size:128" json:"subpage_cron_expr"`
	Timezone       string    `gorm:"size:64" json:"timezone"`
	TimeWindow     string    `gorm:"size:256" json:"time_window"`
	Status         string    `gorm:"size:32;default:enabled" json:"status"`
	LastRunAt      *time.Time `json:"last_run_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AuditLog 审计日志（仅 insert/select，禁止修改删除）。
type AuditLog struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID       int64     `gorm:"index:idx_audit_org_user" json:"user_id"`
	Username     string    `gorm:"size:64;index:idx_audit_org_user" json:"username"`
	Action       string    `gorm:"size:64;index:idx_audit_org_action" json:"action"`
	ResourceType string    `gorm:"size:64;index:idx_audit_org_action" json:"resource_type"`
	ResourceID   string    `gorm:"size:64" json:"resource_id"`
	BeforeValue  string    `gorm:"type:text" json:"before_value"`
	AfterValue   string    `gorm:"type:text" json:"after_value"`
	IP           string    `gorm:"size:64" json:"ip"`
	UserAgent    string    `gorm:"size:512" json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
}

// APIToken API Token（独立于 JWT 的开放集成认证）。
type APIToken struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string     `gorm:"size:128" json:"name"`
	TokenHash  string     `gorm:"size:64;not null" json:"-"`
	Scopes     string     `gorm:"type:text" json:"scopes"`
	Status     string     `gorm:"size:32;default:active" json:"status"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// NotifyChannel 通知渠道。
type NotifyChannel struct {
	ID      int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Type    string `gorm:"size:32" json:"type"`
	Config  string `gorm:"type:text" json:"config"`
	Enabled string `gorm:"size:8;default:true" json:"enabled"`
}

// NotifyRoute 通知路由。
type NotifyRoute struct {
	ID               int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string `gorm:"size:128" json:"name"`
	Rule             string `gorm:"type:text" json:"rule"`
	DefaultChannelID int64  `json:"default_channel_id"`
	Enabled          string `gorm:"size:8;default:true" json:"enabled"`
}

// NoiseRule 降噪规则。
type NoiseRule struct {
	ID      int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Type    string `gorm:"size:32" json:"type"`
	Config  string `gorm:"type:text" json:"config"`
	Enabled string `gorm:"size:8;default:true" json:"enabled"`
}

// ScanWhitelist 扫描授权白名单。
type ScanWhitelist struct {
	ID      int64  `gorm:"primaryKey;autoIncrement" json:"id"`
	Kind    string `gorm:"size:16" json:"kind"` // domain/ip/cidr
	Value   string `gorm:"size:256" json:"value"`
	Remark  string `gorm:"size:512" json:"remark"`
	Enabled string `gorm:"size:8;default:true" json:"enabled"`
}

// WorkerNode Worker 节点。
type WorkerNode struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string     `gorm:"size:128" json:"name"`
	IP               string     `gorm:"size:64" json:"ip"`
	Version          string     `gorm:"size:64" json:"version"`
	Status           string     `gorm:"size:32;default:online" json:"status"`
	HeartbeatAt      *time.Time `json:"heartbeat_at"`
	Load             float64    `gorm:"default:0" json:"load"`
	BootTokenHash    string     `gorm:"size:64" json:"-"`
	ClientID         string     `gorm:"size:128" json:"client_id"`
	ClientSecretHash string     `gorm:"size:128" json:"-"`
	CreatedAt        time.Time  `json:"created_at"`
}

// IntelItem 情报条目（全局）。
type IntelItem struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Source     string    `gorm:"size:16;index:idx_intel_source_sev;not null" json:"source"`
	IntelID    string    `gorm:"size:64;uniqueIndex:idx_intel_id;not null" json:"intel_id"`
	Title      string    `gorm:"size:512" json:"title"`
	Severity   string    `gorm:"size:16;index:idx_intel_source_sev" json:"severity"`
	Scope      string    `gorm:"type:text" json:"scope"`
	TechStack  string    `gorm:"type:text" json:"tech_stack"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// IntelSubscription 情报订阅配置。
type IntelSubscription struct {
	ID         int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Source     string     `gorm:"size:16;uniqueIndex:idx_intel_sub_org_source" json:"source"`
	Enabled    string     `gorm:"size:8;default:true" json:"enabled"`
	LastSyncAt *time.Time `json:"last_sync_at"`
}

// ReportTemplate 报告模板。
type ReportTemplate struct {
	ID        int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string     `gorm:"size:128" json:"name"`
	Sections  string     `gorm:"type:text" json:"sections"`
	Period    string     `gorm:"size:32" json:"period"`
	CronExpr  string     `gorm:"size:128" json:"cron_expr"`
	Timezone  string     `gorm:"size:64" json:"timezone"`
	Enabled   bool       `gorm:"default:false" json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Report 报告。
type Report struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	TemplateID int64     `json:"template_id"`
	Name       string    `gorm:"size:128" json:"name"`
	Title      string    `gorm:"size:256" json:"title"`
	AssetIDs   string    `gorm:"type:text" json:"asset_ids"`
	Period     string    `gorm:"type:text" json:"period"`
	Format     string    `gorm:"size:32;default:pdf" json:"format"`
	Status     string    `gorm:"size:32;default:pending" json:"status"`
	Progress   int       `gorm:"default:0" json:"progress"`
	FilePath   string    `gorm:"size:1024" json:"file_path"`
	Snapshot   string    `gorm:"type:text" json:"snapshot"`
	CreatedAt  time.Time `json:"created_at"`
}

// Webhook 事件推送配置。
type Webhook struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string    `gorm:"size:128" json:"name"`
	URL        string    `gorm:"size:1024" json:"url"`
	SecretHash string    `gorm:"size:128" json:"-"`
	Events     string    `gorm:"type:text" json:"events"`
	Enabled    string    `gorm:"size:8;default:true" json:"enabled"`
	LastStatus string    `gorm:"size:32" json:"last_status"`
	LastError  string    `gorm:"size:1024" json:"last_error"`
	RetryCount int       `json:"retry_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// AvailabilityPoint 可用性时序点（HTTP/DNS/TCP/PING）。
type AvailabilityPoint struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID    int64     `gorm:"index:idx_avail_org_asset_ts" json:"asset_id"`
	Engine     string    `gorm:"size:16" json:"engine"`
	StatusCode int       `json:"status_code"`
	ResponseMs int       `json:"response_ms"`
	SampledAt  time.Time `gorm:"index:idx_avail_org_asset_ts" json:"sampled_at"`
}

// TrendPoint 趋势点。
type TrendPoint struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Metric    string    `gorm:"size:64" json:"metric"`
	Value     float64   `json:"value"`
	SampledAt time.Time `gorm:"index:idx_trend_org_metric_sampled" json:"sampled_at"`
}

// EscalationRule 告警升级规则。
type EscalationRule struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"size:128" json:"name"`
	Trigger   string    `gorm:"size:64" json:"trigger"`
	EscalateTo string   `gorm:"size:64" json:"escalate_to"`
	Enabled   bool      `gorm:"default:true" json:"enabled"`
	Version   int       `gorm:"default:1" json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

// WatchShift 值守班次。
type WatchShift struct {
	ID           int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	StartTime    time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
	OnDuty       string    `gorm:"size:128" json:"on_duty"`
	HandoverNote string    `gorm:"type:text" json:"handover_note"`
	Status       string    `gorm:"size:16;default:active;index:idx_shift_org_status" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// DailyWarReport 每日战报。
type DailyWarReport struct {
	ID         int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	ReportDate string    `gorm:"size:32;uniqueIndex:idx_war_report_org_date" json:"report_date"`
	Summary    string    `gorm:"type:text" json:"summary"`
	FilePath   string    `gorm:"size:1024" json:"file_path"`
	CreatedAt  time.Time `json:"created_at"`
}

// Scenario 扫描场景（预置触发规则）。
type Scenario struct {
	ID               int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	Name             string     `gorm:"size:128" json:"name"`
	ScenarioType     string     `gorm:"size:32" json:"scenario_type"`
	Description      string     `gorm:"size:1024" json:"description"`
	PolicyID         int64      `gorm:"default:0" json:"policy_id"`
	AssetGroupName   string     `gorm:"size:128" json:"asset_group_name"`
	Activated        bool       `gorm:"default:false" json:"activated"`
	ActivatedAt      *time.Time `json:"activated_at"`
	DeactivatedAt    *time.Time `json:"deactivated_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// SOPTemplate 应急响应 SOP 模板（JSON 存于规则库）。
type SOPTemplate struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	EventType string    `gorm:"size:64" json:"event_type"`
	Title     string    `gorm:"size:256" json:"title"`
	Steps     string    `gorm:"type:text" json:"steps"`
	CreatedAt time.Time `json:"created_at"`
}

// ContentBaseline 内容完整性基线：纳入监测的重点资产内容指纹（标题/正文/HTML Hash）。
type ContentBaseline struct {
	ID              int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID         int64     `gorm:"index:idx_cb_org_asset;not null" json:"asset_id"`
	URL             string    `gorm:"size:1024;not null" json:"url"`
	Fingerprint     string    `gorm:"type:text" json:"fingerprint"`
	FingerprintVer  string    `gorm:"size:16;default:v1" json:"fingerprint_version"`
	FirstSeenAt     time.Time `json:"first_seen_at"`
	LastSeenAt      time.Time `json:"last_seen_at"`
	ChangedAt       *time.Time `json:"changed_at"`
	ChangedCount    int       `gorm:"default:0" json:"changed_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ExternalLinkBaseline 外链发现基线：资产页面的外链清单（JSON），比对新增/移除/目标域名变更。
type ExternalLinkBaseline struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	AssetID     int64     `gorm:"index:idx_elb_org_asset;not null" json:"asset_id"`
	URL         string    `gorm:"size:1024;not null" json:"url"`
	Links       string    `gorm:"type:text" json:"links"` // JSON: [{"url","type","domain"}]
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ChangedAt   *time.Time `json:"changed_at"`
	ChangedCount int      `gorm:"default:0" json:"changed_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// License 授权导入记录（审计用，授权真相源为授权文件）。
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
