package service

import (
	"fmt"
	"os"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// SeedService 首启初始化：admin、默认策略模板、默认降噪规则、默认场景。
type SeedService struct {
	DB        *gorm.DB
	AdminUser string
	AdminPass string
}

// NewSeedService 构造 SeedService。
func NewSeedService(db *gorm.DB, adminUser, adminPass string) *SeedService {
	return &SeedService{DB: db, AdminUser: adminUser, AdminPass: adminPass}
}

// EnsureAdmin 确保 admin 存在；首次创建时密码随机生成并打印一次。
// 返回 (已初始化, 新生成密码或空)。
func (s *SeedService) EnsureAdmin() (bool, string, error) {
	var count int64
	if err := s.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count).Error; err != nil {
		return false, "", err
	}
	if count > 0 {
		return true, "", nil
	}
	pwd := s.AdminPass
	if pwd == "" {
		pwd = utils.SHA256HexString(fmt.Sprintf("%d-%s", os.Getpid(), "admin-init"))[:16]
	}
	hash, err := utils.HashPassword(pwd)
	if err != nil {
		return false, "", err
	}
	u := &models.User{
		Username: s.AdminUser,
		Password: hash,
		Status:   models.StatusActive,
		Role:     models.RoleAdmin,
	}
	if err := s.DB.Create(u).Error; err != nil {
		return false, "", err
	}
	return false, pwd, nil
}

// EnsureDefaults 初始化默认策略模板与降噪规则。
func (s *SeedService) EnsureDefaults() error {
	// 默认策略模板。
	var count int64
	s.DB.Model(&models.ScanPolicy{}).Count(&count)
	if count == 0 {
		s.DB.Create(&models.ScanPolicy{
			Name: "默认日常巡检", Scenario: "daily",
			EngineSwitches: `{"availability":{"enabled":true,"fail_count":3,"slow_threshold_ms":3000},"vuln_scan":{"enabled":true},"content":{"enabled":true},"hidden_link":{"enabled":true},"webshell":{"enabled":true},"phishing":{"enabled":true},"port_service":{"enabled":true},"dns_security":{"enabled":true},"reputation":{"enabled":true},"intelligence":{"enabled":true}}`,
			Concurrency: 4, Timeout: 60, RateLimit: 10, ScanDepth: 2, ConcurrencyLimit: 4, SameOrigin: true, CrawlSubpages: true,
		})
	}
	// 默认降噪规则：本地/内网回环地址白名单，避免可用性误报。
	var rules int64
	s.DB.Model(&models.NoiseRule{}).Where("type = ?", "whitelist_ip").Count(&rules)
	if rules == 0 {
		s.DB.Create(&models.NoiseRule{
			Type:    "whitelist_ip",
			Config:  `{"ip":"127.0.0.1","remark":"本地回环地址"}`,
			Enabled: "true",
		})
	}
	return nil
}

// IsInitialized 平台是否已初始化（用于禁用未初始化平台管理）。
func (s *SeedService) IsInitialized() bool {
	var count int64
	s.DB.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count)
	return count > 0
}
