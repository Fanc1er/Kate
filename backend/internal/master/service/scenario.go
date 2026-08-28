package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// ScenarioService 扫描场景管理：预置触发规则，激活时按资产组 + 策略批量创建任务。
type ScenarioService struct {
	DB   *gorm.DB
	Task *TaskService
}

// NewScenarioService 构造 ScenarioService。
func NewScenarioService(db *gorm.DB, task *TaskService) *ScenarioService {
	return &ScenarioService{DB: db, Task: task}
}

// List 列出所有场景。
func (s *ScenarioService) List() ([]models.Scenario, error) {
	var list []models.Scenario
	if err := s.DB.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create 新建场景（policy_id 须存在且 > 0）。
func (s *ScenarioService) Create(name, scenarioType, description string, policyID int64, assetGroupName string, activated bool) (*models.Scenario, error) {
	if name == "" {
		return nil, errs.New(errs.CodeValidationFailed, "场景名称必填")
	}
	if policyID <= 0 {
		return nil, errs.New(errs.CodeValidationFailed, "策略模板必填")
	}
	var policy models.ScanPolicy
	if err := s.DB.Where("id = ?", policyID).First(&policy).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errs.New(errs.CodeNotFound, "策略模板不存在")
		}
		return nil, err
	}
	sc := &models.Scenario{
		Name:             name,
		ScenarioType:     scenarioType,
		Description:      description,
		PolicyID:         policyID,
		AssetGroupName:   assetGroupName,
		Activated:        activated,
		ActivatedAt:      nil,
		DeactivatedAt:    nil,
	}
	now := time.Now()
	if activated {
		sc.ActivatedAt = &now
	}
	if err := s.DB.Create(sc).Error; err != nil {
		return nil, err
	}
	return sc, nil
}

// Update 更新场景配置。
func (s *ScenarioService) Update(id int64, name, scenarioType, description *string, policyID *int64, assetGroupName *string) (*models.Scenario, error) {
	var sc models.Scenario
	if err := s.DB.First(&sc, id).Error; err != nil {
		return nil, err
	}
	updates := map[string]any{}
	if name != nil {
		updates["name"] = *name
	}
	if scenarioType != nil {
		updates["scenario_type"] = *scenarioType
	}
	if description != nil {
		updates["description"] = *description
	}
	if policyID != nil {
		if *policyID <= 0 {
			return nil, errs.New(errs.CodeValidationFailed, "策略模板必填")
		}
		var policy models.ScanPolicy
		if err := s.DB.Where("id = ?", *policyID).First(&policy).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errs.New(errs.CodeNotFound, "策略模板不存在")
			}
			return nil, err
		}
		updates["policy_id"] = *policyID
	}
	if assetGroupName != nil {
		updates["asset_group_name"] = *assetGroupName
	}
	if len(updates) > 0 {
		if err := s.DB.Model(&sc).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	return &sc, nil
}

// Delete 删除场景。
func (s *ScenarioService) Delete(id int64) error {
	return s.DB.Delete(&models.Scenario{}, id).Error
}

// ToggleActivate 切换场景激活状态（激活时批量为资产组内活跃资产创建任务）。
func (s *ScenarioService) ToggleActivate(id int64, activate bool) (*models.Scenario, error) {
	var sc models.Scenario
	if err := s.DB.First(&sc, id).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	updates := map[string]any{}
	if activate {
		updates["activated"] = true
		updates["activated_at"] = now
		updates["deactivated_at"] = nil
	} else {
		updates["activated"] = false
		updates["activated_at"] = nil
		updates["deactivated_at"] = now
	}
	if err := s.DB.Model(&sc).Updates(updates).Error; err != nil {
		return nil, err
	}
	// 激活时：按资产组筛选活跃资产，批量创建任务。
	if activate && s.Task != nil && sc.PolicyID > 0 {
		var assetIDs []int64
		q := s.DB.Model(&models.Asset{}).Where("status <> ?", "deleted")
		if sc.AssetGroupName != "" {
			q = q.Where("group_name = ?", sc.AssetGroupName)
		}
		if err := q.Pluck("id", &assetIDs).Error; err != nil {
			return nil, err
		}
		if len(assetIDs) > 0 {
			if _, err := s.Task.Create(TaskCreateReq{AssetIDs: assetIDs, PolicyID: sc.PolicyID}, 0, "system", "", ""); err != nil {
				// 任务创建冲突（已存在进行中任务）时静默跳过，视为本次已尝试。
				return nil, errs.New(errs.CodeTaskStateConflict, err.Error())
			}
		}
	}
	// 重新查询最新状态。
	if err := s.DB.First(&sc, id).Error; err != nil {
		return nil, err
	}
	return &sc, nil
}
