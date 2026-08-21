package service

import (
	"fmt"
	"os"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// SeedService 首启初始化：super_admin、默认策略模板、默认降噪规则、默认场景。
type SeedService struct {
	DB        *gorm.DB
	AdminUser string
	AdminPass string
}

// NewSeedService 构造 SeedService。
func NewSeedService(db *gorm.DB, adminUser, adminPass string) *SeedService {
	return &SeedService{DB: db, AdminUser: adminUser, AdminPass: adminPass}
}

// EnsureSuperAdmin 确保 super_admin 存在；首次创建时密码随机生成并打印一次。
// 返回 (已初始化, 新生成密码或空)。
func (s *SeedService) EnsureSuperAdmin() (bool, string, error) {
	var count int64
	if err := s.DB.Model(&models.User{}).Where("is_super_admin = ?", true).Count(&count).Error; err != nil {
		return false, "", err
	}
	if count > 0 {
		return true, "", nil
	}
	pwd := s.AdminPass
	if pwd == "" {
		pwd = utils.SHA256HexString(fmt.Sprintf("%d-%s", os.Getpid(), "super-admin-init"))[:16]
	}
	hash, err := utils.HashPassword(pwd)
	if err != nil {
		return false, "", err
	}
	u := &models.User{
		Username:     s.AdminUser,
		Password:     hash,
		Status:       models.StatusActive,
		IsSuperAdmin: true,
	}
	if err := s.DB.Create(u).Error; err != nil {
		return false, "", err
	}
	return false, pwd, nil
}

// EnsureDefaults 初始化默认策略模板与降噪规则（平台级 org_id=0 模板，按组织复用）。
func (s *SeedService) EnsureDefaults() error {
	// 默认策略模板（全组织通用，org_id=0 表示平台级模板，任务创建时复制到组织）。
	var count int64
	s.DB.Model(&models.ScanPolicy{}).Where("org_id = ?", 0).Count(&count)
	if count == 0 {
		s.DB.Create(&models.ScanPolicy{
			OrgID: 0, Name: "默认日常巡检", Scenario: "daily",
			EngineSwitches: `{"availability":{"enabled":true,"fail_count":3,"slow_threshold_ms":3000},"vuln_scan":{"enabled":true},"content_security":{"enabled":true}}`,
			Concurrency: 4, Timeout: 60, RateLimit: 10, ScanDepth: 2, ConcurrencyLimit: 4, SameOrigin: true, CrawlSubpages: true,
		})
	}
	// 默认降噪规则（平台级：本地/内网回环地址白名单，避免可用性误报）。
	var rules int64
	s.DB.Model(&models.NoiseRule{}).Where("org_id = ? AND type = ?", 0, "whitelist_ip").Count(&rules)
	if rules == 0 {
		s.DB.Create(&models.NoiseRule{
			OrgID: 0, Type: "whitelist_ip",
			Config:  `{"ip":"127.0.0.1","remark":"本地回环地址"}`,
			Enabled: "true",
		})
	}
	return nil
}

// IsInitialized 平台是否已初始化（用于禁用未初始化平台管理）。
func (s *SeedService) IsInitialized() bool {
	var count int64
	s.DB.Model(&models.User{}).Where("is_super_admin = ?", true).Count(&count)
	return count > 0
}
