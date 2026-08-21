package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/internal/master/repository"
	"github.com/Fanc1er/Kate/backend/pkg/badger"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// AssetService 资产服务：CRUD + URL 归一化去重 + 画像 + 变更追踪 + 批量操作。
type AssetService struct {
	DB     *gorm.DB
	Cache  *badger.Store
	Audit  *AuditWriter
	Tokens *TokenManager
}

// NewAssetService 构造 AssetService。
func NewAssetService(db *gorm.DB, cache *badger.Store, audit *AuditWriter) *AssetService {
	return &AssetService{DB: db, Cache: cache, Audit: audit}
}

// guard 返回组织隔离守卫（org_id 强制过滤，缺省 org_id 拒绝查询）。
func (s *AssetService) guard(orgID int64) *repository.Guard {
	g, err := repository.NewGuard(s.DB, orgID)
	if err != nil {
		panic(err)
	}
	return g
}

// urlKey 归一化 URL 的 BadgerDB 去重键。
func urlKey(orgID int64, md5 string) string {
	return fmt.Sprintf("urlmd5:%d:%s", orgID, md5)
}

// Create 创建资产（URL 归一化 + BadgerDB MD5 防重 + 配额校验）。
func (s *AssetService) Create(orgID int64, name, rawURL, group, importance, remark string, userID int64, username, ip, ua string) (*models.Asset, error) {
	normalized, err := utils.NormalizeURL(rawURL)
	if err != nil {
		return nil, errs.New(errs.CodeValidationFailed, "URL 不合法")
	}
	// 配额校验。
	if err := s.checkQuota(orgID); err != nil {
		return nil, err
	}
	key := urlKey(orgID, utils.URLKey(normalized))
	exists, err := s.Cache.Has(key)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errs.New(errs.CodeDuplicateURL, "该 URL 已存在")
	}
	if importance == "" {
		importance = "medium"
	}
	asset := &models.Asset{
		OrgID: orgID, URL: normalized, Name: name, GroupName: group,
		Importance: importance, Remark: remark, Status: models.StatusActive, SourceType: "manual",
	}
	if err := s.DB.Create(asset).Error; err != nil {
		return nil, err
	}
	_ = s.Cache.Set(key, strconv.FormatInt(asset.ID, 10))
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "asset.create", "asset", fmt.Sprint(asset.ID), "", normalized, ip, ua)
	}
	return asset, nil
}

func (s *AssetService) checkQuota(orgID int64) error {
	var org models.Organization
	if err := s.DB.First(&org, orgID).Error; err != nil {
		return err
	}
	var used int64
	s.DB.Model(&models.Asset{}).Where("org_id = ? AND status <> ?", orgID, models.StatusDeleted).Count(&used)
	if int(used) >= org.MaxAssets {
		return errs.New(errs.CodeAssetQuota, "")
	}
	return nil
}

// Update 编辑资产（带乐观锁 version）。
func (s *AssetService) Update(orgID, id int64, patch map[string]any, version int, userID int64, username, ip, ua string) (*models.Asset, error) {
	var asset models.Asset
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&asset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	if version > 0 && patch["version"] == nil {
		patch["version"] = version
	}
	if v, ok := patch["version"]; ok {
		vv := int(v.(float64))
		if vv != asset.Version {
			return nil, errs.New(errs.CodeTaskStateConflict, "数据已被他人修改，请刷新后重试")
		}
		patch["version"] = asset.Version + 1
	} else {
		patch["version"] = asset.Version + 1
	}
	before, _ := json.Marshal(asset)
	if err := s.DB.Model(&asset).Updates(patch).Error; err != nil {
		return nil, err
	}
	s.DB.First(&asset, asset.ID)
	if s.Audit != nil {
		after, _ := json.Marshal(asset)
		s.Audit.Write(orgID, userID, username, "asset.update", "asset", fmt.Sprint(id), string(before), string(after), ip, ua)
	}
	return &asset, nil
}

// Delete 删除资产（软删除 status=deleted + 清理 BadgerDB 键）。
func (s *AssetService) Delete(orgID, id int64, userID int64, username, ip, ua string) error {
	var asset models.Asset
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&asset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errs.New(errs.CodeNotFound, "")
		}
		return err
	}
	if err := s.DB.Model(&asset).Updates(map[string]any{"status": models.StatusDeleted}).Error; err != nil {
		return err
	}
	_ = s.Cache.Delete(urlKey(orgID, utils.URLKey(asset.URL)))
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "asset.delete", "asset", fmt.Sprint(id), asset.URL, "deleted", ip, ua)
	}
	return nil
}

// List 资产列表（模糊搜索/分组/重要程度/状态筛选 + 分页）。
func (s *AssetService) List(orgID int64, keyword, group, importance, status, sourceType string, page, pageSize int) ([]models.Asset, int64, error) {
	q := s.guard(orgID).Scoped(&models.Asset{}).Where("status <> ?", models.StatusDeleted)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("(url LIKE ? OR name LIKE ?)", like, like)
	}
	if group != "" {
		q = q.Where("group_name = ?", group)
	}
	if importance != "" {
		q = q.Where("importance = ?", importance)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if sourceType != "" {
		q = q.Where("source_type = ?", sourceType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Asset
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Groups 分组去重计数。
func (s *AssetService) Groups(orgID int64) ([]GroupStat, error) {
	type row struct {
		GroupName string
		Cnt       int64
	}
	var rows []row
	if err := s.DB.Model(&models.Asset{}).
		Select("group_name, COUNT(*) AS cnt").
		Where("org_id = ? AND status <> ? AND group_name <> ''", orgID, models.StatusDeleted).
		Group("group_name").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]GroupStat, 0, len(rows))
	for _, r := range rows {
		out = append(out, GroupStat{Group: r.GroupName, Count: r.Cnt})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

// GroupStat 分组统计。
type GroupStat struct {
	Group string `json:"group_name"`
	Count int64  `json:"count"`
}

// Get 资产详情。
func (s *AssetService) Get(orgID, id int64) (*models.Asset, error) {
	var a models.Asset
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&a).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	return &a, nil
}

// Profile 资产画像（技术栈指纹/ICP/SSL 倒计时/端口快照）。
func (s *AssetService) Profile(orgID, id int64) (map[string]any, error) {
	a, err := s.Get(orgID, id)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"asset":        a,
		"tech_stack":   a.TechStack,
		"icp":          a.ICP,
		"ssl_expire":   a.SSLExpire,
		"subdomains":   []string{},
		"port_snapshot": []string{},
	}, nil
}

// History 变更追踪（读取审计日志中 asset.create/update/delete 记录）。
func (s *AssetService) History(orgID, id int64) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	if err := s.DB.Where("org_id = ? AND resource_type = ? AND resource_id = ?", orgID, "asset", fmt.Sprint(id)).
		Order("created_at DESC").Limit(50).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}

// BatchDelete 批量删除（仅 org_admin，见权限矩阵）。
func (s *AssetService) BatchDelete(orgID int64, ids []int64, userID int64, username, ip, ua string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if err := s.Delete(orgID, id, userID, username, ip, ua); err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// BatchGroup 批量改分组。
func (s *AssetService) BatchGroup(orgID int64, ids []int64, group string) BatchResult {
	res := BatchResult{Success: 0}
	for _, id := range ids {
		if err := s.DB.Model(&models.Asset{}).Where("id = ? AND org_id = ?", id, orgID).
			Update("group_name", group).Error; err != nil {
			res.Failed = append(res.Failed, FailedItem{ID: id, Reason: errs.FromError(err).Message})
		} else {
			res.Success++
		}
	}
	return res
}

// BatchImport 批量导入（URL 列表或 CSV）。
func (s *AssetService) BatchImport(orgID int64, content string, userID int64, username, ip, ua string) (ImportResult, error) {
	lines := parseImportLines(content)
	res := ImportResult{}
	for i, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, ",")
		url := strings.TrimSpace(fields[0])
		name := ""
		if len(fields) > 1 {
			name = strings.TrimSpace(fields[1])
		}
		if url == "" {
			continue
		}
		_, err := s.Create(orgID, name, url, "", "medium", "", userID, username, ip, ua)
		if err != nil {
			res.Failed++
			res.Errors = append(res.Errors, fmt.Sprintf("第 %d 行 %s: %s", i+1, url, errs.FromError(err).Message))
		} else {
			res.Success++
		}
	}
	return res, nil
}

func parseImportLines(content string) []string {
	var out []string
	content = strings.TrimSpace(content)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

// ImportResult 导入结果。
type ImportResult struct {
	Success int      `json:"success"`
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
}

// BatchResult 批量操作结果。
type BatchResult struct {
	Success int          `json:"success"`
	Failed  []FailedItem `json:"failed,omitempty"`
}

// FailedItem 批量失败条目。
type FailedItem struct {
	ID     int64  `json:"id"`
	Reason string `json:"reason"`
}

// ExportCSV 当前筛选结果导出 CSV。
func (s *AssetService) ExportCSV(orgID int64, keyword, group, importance, status string) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"ID", "URL", "名称", "分组", "重要程度", "状态", "创建时间"})
	q := s.guard(orgID).Scoped(&models.Asset{}).Where("status <> ?", models.StatusDeleted)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("(url LIKE ? OR name LIKE ?)", like, like)
	}
	if group != "" {
		q = q.Where("group_name = ?", group)
	}
	if importance != "" {
		q = q.Where("importance = ?", importance)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var list []models.Asset
	if err := q.Order("id DESC").Limit(5000).Find(&list).Error; err != nil {
		return nil, err
	}
	for _, a := range list {
		_ = w.Write([]string{
			strconv.FormatInt(a.ID, 10), a.URL, a.Name, a.GroupName, a.Importance, a.Status,
			a.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	w.Flush()
	return []byte(buf.String()), nil
}

// WechatAsset CRUD
func (s *AssetService) CreateWechat(orgID int64, m map[string]any) (*models.WechatAsset, error) {
	a := &models.WechatAsset{OrgID: orgID}
	if v, ok := m["name"]; ok {
		a.Name = v.(string)
	}
	if v, ok := m["wechat_id"]; ok {
		a.WechatID = v.(string)
	}
	if v, ok := m["avatar_url"]; ok {
		a.AvatarURL = v.(string)
	}
	if v, ok := m["intro"]; ok {
		a.Intro = v.(string)
	}
	if v, ok := m["verify_status"]; ok {
		a.VerifyStatus = v.(string)
	}
	a.FansCount = intField(m, "fans_count")
	a.ArticleCount = intField(m, "article_count")
	if err := s.DB.Create(a).Error; err != nil {
		return nil, err
	}
	return a, nil
}

// intField 从 map 安全读取整型字段（缺省/非数字返回 0，避免类型断言 panic）。
func intField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		var n int
		fmt.Sscanf(v, "%d", &n)
		return n
	default:
		return 0
	}
}

func (s *AssetService) ListWechat(orgID int64, page, pageSize int) ([]models.WechatAsset, int64, error) {
	q := s.DB.Model(&models.WechatAsset{}).Where("org_id = ?", orgID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.WechatAsset
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *AssetService) UpdateWechat(orgID, id int64, m map[string]any) (*models.WechatAsset, error) {
	var a models.WechatAsset
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&a).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, errs.New(errs.CodeNotFound, "")
		}
		return nil, err
	}
	if err := s.DB.Model(&a).Updates(m).Error; err != nil {
		return nil, err
	}
	s.DB.First(&a, a.ID)
	return &a, nil
}

func (s *AssetService) DeleteWechat(orgID, id int64) error {
	return s.DB.Where("id = ? AND org_id = ?", id, orgID).Delete(&models.WechatAsset{}).Error
}
