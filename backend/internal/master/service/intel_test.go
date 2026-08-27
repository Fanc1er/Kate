package service

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func newIntelSvc(t *testing.T) *IntelService {
	t.Helper()
	return NewIntelService(newTestDB(t), nil)
}

func TestIntelImportAndList(t *testing.T) {
	svc := newIntelSvc(t)
	n, err := svc.Import([]IntelInput{
		{IntelID: "CVE-2024-0001", Title: "测试组件 RCE", Severity: "critical", Component: "nginx", MaxVersion: "1.18.0"},
		{IntelID: "CVE-2024-0002", Title: "仅展示条目"},
	})
	if err != nil || n != 2 {
		t.Fatalf("导入 2 条: n=%d err=%v", n, err)
	}
	list, total, err := svc.List(1, 10)
	if err != nil || total != 2 {
		t.Fatalf("List total=%d err=%v", total, err)
	}
	if list[0].Source != "manual" {
		t.Fatalf("source = %s, want manual", list[0].Source)
	}
}

func TestIntelImportUpsertByIdempotent(t *testing.T) {
	svc := newIntelSvc(t)
	_, _ = svc.Import([]IntelInput{{IntelID: "CVE-2024-0009", Title: "旧标题", Component: "tomcat", MaxVersion: "9.0.30"}})
	n, err := svc.Import([]IntelInput{{IntelID: "CVE-2024-0009", Title: "新标题", Severity: "low", Component: "tomcat", MaxVersion: "9.0.31"}})
	if err != nil || n != 1 {
		t.Fatalf("重复导入应覆盖 1 条: n=%d err=%v", n, err)
	}
	var items []models.IntelItem
	if err := svc.DB.Where("intel_id = ?", "CVE-2024-0009").Find(&items).Error; err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("intel_id 唯一去重失败, got %d 条", len(items))
	}
	if items[0].Title != "新标题" || items[0].Scope != "9.0.31" {
		t.Fatalf("覆盖更新未生效: %+v", items[0])
	}
}

func TestIntelImportValidation(t *testing.T) {
	svc := newIntelSvc(t)
	if _, err := svc.Import([]IntelInput{{Title: "缺少编号"}}); err == nil {
		t.Fatalf("缺 intel_id 应报校验错误")
	}
	if _, err := svc.Import([]IntelInput{{IntelID: "X", Title: "t", Severity: "ultra"}}); err == nil {
		t.Fatalf("非法 severity 应报校验错误")
	}
}

func TestLoadComponentRulesFiltersEmpty(t *testing.T) {
	db := newTestDB(t)
	svc := NewIntelService(db, nil)
	_, _ = svc.Import([]IntelInput{
		{IntelID: "CVE-2024-0011", Title: "nginx 漏洞", Component: "nginx", MaxVersion: "1.18.0"},
		{IntelID: "CVE-2024-0012", Title: "无组件条目"},
	})
	rules := LoadComponentRules(db, 100)
	if len(rules) != 1 {
		t.Fatalf("应只下发带组件与版本的条目, got %d", len(rules))
	}
	if rules[0]["component"] != "nginx" || rules[0]["max_version"] != "1.18.0" {
		t.Fatalf("规则字段映射不符: %v", rules[0])
	}
}

func TestIntelDelete(t *testing.T) {
	svc := newIntelSvc(t)
	_, _ = svc.Import([]IntelInput{{IntelID: "CVE-2024-0021", Title: "待删除"}})
	list, _, _ := svc.List(1, 10)
	if err := svc.Delete(list[0].ID, 1, "tester", "127.0.0.1", "test"); err != nil {
		t.Fatalf("Delete err: %v", err)
	}
	_, total, _ := svc.List(1, 10)
	if total != 0 {
		t.Fatalf("删除后 total = %d, want 0", total)
	}
}
