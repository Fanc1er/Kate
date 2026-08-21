// Package config 负责从环境变量加载 Master/Worker 运行配置。
package config

import (
	"os"
	"strconv"
	"time"
)

// Config 汇总 Master 与 Worker 的运行配置，全部来自环境变量，缺省使用设计文档默认值。
type Config struct {
	Port            string        // Master HTTP 端口
	DBPath          string        // SQLite 路径
	DataDir         string        // 证据/规则/日志根目录
	JWTSecret       string        // JWT 签名密钥（必填）
	JWTTTL          time.Duration // access token 有效期
	RefreshTTL      time.Duration // refresh token 有效期
	SuperAdminUser  string        // 初始超管用户名
	SuperAdminPass  string        // 初始超管密码（空则随机生成）
	RulesDir        string        // fsnotify 规则目录
	SwaggerEnabled  bool          // 是否暴露 /swagger/*
	Timezone        string        // 定时任务 Cron 时区
	WorkerPollMS    int           // Worker 任务轮询间隔（毫秒）
	WorkerHeartbeat int           // Worker 心跳间隔（毫秒）
	AntConcurrency  int           // Worker ants 协程池大小
	StormLimitHour  int           // 单资产每小时告警上限
	StealthMode     bool          // 低速隐蔽模式
	ProxyURL        string        // 反封禁代理
	HarMaxBody      int           // HAR 响应体截断上限（字节）
	ScreenshotOn    bool          // Worker 无头浏览器截图开关
	ScreenshotConc  int           // 截图并发上限
	EvidenceTTL     int           // 证据文件保留天数
	ChannelKey      string        // 通知渠道加密主密钥（AES-256-GCM）
	SMTPHost        string
	SMTPPort        int
	SMTPUser        string
	SMTPPassword    string
	SMTPFrom        string
	MailCodeTTL     time.Duration // 邮件验证码有效期
}

// Load 读取环境变量构建 Config。
func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DBPath:          getEnv("CINSIGHT_DB_PATH", "./data/cinsight.db"),
		DataDir:         getEnv("CINSIGHT_DATA_DIR", "./data"),
		JWTSecret:       os.Getenv("CINSIGHT_JWT_SECRET"),
		JWTTTL:          getDuration("CINSIGHT_JWT_TTL", 15*time.Minute),
		RefreshTTL:      getDuration("CINSIGHT_REFRESH_TTL", 7*24*time.Hour),
		SuperAdminUser:  getEnv("CINSIGHT_SUPER_ADMIN_USER", "admin"),
		SuperAdminPass:  os.Getenv("CINSIGHT_SUPER_ADMIN_PASS"),
		RulesDir:        getEnv("CINSIGHT_RULES_DIR", "./data/rules"),
		SwaggerEnabled:  getBool("CINSIGHT_SWAGGER_ENABLED", false),
		Timezone:        getEnv("CINSIGHT_TIMEZONE", "Asia/Shanghai"),
		WorkerPollMS:    getInt("CINSIGHT_WORKER_POLL_MS", 3000),
		WorkerHeartbeat: getInt("CINSIGHT_WORKER_HEARTBEAT_MS", 5000),
		AntConcurrency:  getInt("CINSIGHT_ANT_CONCURRENCY", 20),
		StormLimitHour:  getInt("CINSIGHT_STORM_LIMIT_PER_HOUR", 5),
		StealthMode:     getBool("CINSIGHT_STEALTH_MODE", false),
		ProxyURL:        os.Getenv("CINSIGHT_PROXY_URL"),
		HarMaxBody:      getInt("CINSIGHT_HAR_MAX_BODY", 1<<20),
		ScreenshotOn:    getBool("CINSIGHT_SCREENSHOT_ENABLED", true),
		ScreenshotConc:  getInt("CINSIGHT_SCREENSHOT_CONCURRENCY", 2),
		EvidenceTTL:     getInt("CINSIGHT_EVIDENCE_TTL_DAYS", 365),
		ChannelKey:      os.Getenv("CINSIGHT_CHANNEL_KEY"),
		SMTPHost:        os.Getenv("CINSIGHT_SMTP_HOST"),
		SMTPPort:        getInt("CINSIGHT_SMTP_PORT", 465),
		SMTPUser:        os.Getenv("CINSIGHT_SMTP_USER"),
		SMTPPassword:    os.Getenv("CINSIGHT_SMTP_PASSWORD"),
		SMTPFrom:        os.Getenv("CINSIGHT_SMTP_FROM"),
		MailCodeTTL:     getDuration("CINSIGHT_MAIL_CODE_TTL", 5*time.Minute),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
