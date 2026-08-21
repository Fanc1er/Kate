package service

import (
	"encoding/json"
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func TestPullTaskKeywordRules(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	policy := models.ScanPolicy{OrgID: 1, Name: "p", Concurrency: 4, Timeout: 60, ScanDepth: 2, ConcurrencyLimit: 4, SameOrigin: true, CrawlSubpages: true}
	if err := gdb.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: policy.ID, Status: models.StatusPending}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}
	// 组织自定义 keyword 规则 + 全局 keyword 规则。
	rules := []models.RuleDefinition{
		{OrgID: 1, Kind: "keyword", Name: "竞品词", FRegex: `超级竞品`, Sensitive: false, Loaded: true},
		{OrgID: 1, Kind: "keyword", Name: "违规词", FRegex: `违规内容`, SRegex: `公司活动`, Sensitive: true, Loaded: true},
		{OrgID: 1, Kind: "keyword", Name: "未加载", FRegex: `x`, Loaded: false},
		{OrgID: 1, Kind: "sensitive", Name: "非关键词", FRegex: `身份证`, Loaded: true},
	}
	for i := range rules {
		if err := gdb.Create(&rules[i]).Error; err != nil {
			t.Fatalf("create rule: %v", err)
		}
	}
	// gorm 默认值 default:true 会覆盖零值 false，显式回写未加载标志。
	if err := gdb.Model(&models.RuleDefinition{}).Where("id = ?", rules[2].ID).Update("loaded", false).Error; err != nil {
		t.Fatalf("set unloaded: %v", err)
	}

	data, err := s.PullTask(1, "worker-1")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	if data["task"] == nil {
		t.Fatal("task 不应为 nil")
	}
	raw, _ := json.Marshal(data["keyword_rules"])
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal keyword_rules: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("应下发 2 条 keyword 规则（loaded 且 kind=keyword），got %d: %v", len(out), out)
	}
	// 敏感级标志与过滤正则应透传。
	got := map[string]any{}
	for _, r := range out {
		got[r["name"].(string)] = r
	}
	if r, ok := got["违规词"]; !ok {
		t.Fatal("应包含敏感规则")
	} else {
		rm, ok := r.(map[string]any)
		if !ok {
			t.Fatalf("规则结构非法: %T", r)
		}
		if rm["sensitive"] != true {
			t.Fatal("sensitive 标志应透传 true")
		}
		if rm["s_regex"] != "公司活动" {
			t.Fatalf("s_regex 应透传, got %v", rm["s_regex"])
		}
	}
}

func TestPullTaskEmptyKeywordRules(t *testing.T) {
	gdb := newTestDB(t)
	assessor := NewResultAssessor(gdb)
	taskSvc := NewTaskService(gdb, nil, assessor)
	s := NewWorkerService(gdb, taskSvc, nil, NewHub(), 10)

	asset := models.Asset{OrgID: 1, URL: "https://example.com", Name: "example"}
	if err := gdb.Create(&asset).Error; err != nil {
		t.Fatalf("create asset: %v", err)
	}
	policy := models.ScanPolicy{OrgID: 1, Name: "p", Concurrency: 4, Timeout: 60, ScanDepth: 2, ConcurrencyLimit: 4, SameOrigin: true, CrawlSubpages: true}
	if err := gdb.Create(&policy).Error; err != nil {
		t.Fatalf("create policy: %v", err)
	}
	task := models.ScanTask{OrgID: 1, AssetID: asset.ID, PolicyID: policy.ID, Status: models.StatusPending}
	if err := gdb.Create(&task).Error; err != nil {
		t.Fatalf("create task: %v", err)
	}

	data, err := s.PullTask(1, "worker-1")
	if err != nil {
		t.Fatalf("pull: %v", err)
	}
	raw, _ := json.Marshal(data["keyword_rules"])
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("无 keyword 规则时应下发空数组, got %d", len(out))
	}
}
