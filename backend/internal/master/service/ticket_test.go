package service

import (
	"testing"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
)

func newTestTriage(t *testing.T) *TriageService {
	t.Helper()
	gdb := newTestDB(t)
	return NewTriageService(gdb, nil, nil)
}

// TestCreateTicketValidation 事件/漏洞来源至少其一非空。
func TestCreateTicketValidation(t *testing.T) {
	s := newTestTriage(t)
	if _, err := s.CreateTicket(1, 0, 0, "alice", "note", nil, 1, "u", "ip", "ua"); err == nil {
		t.Fatal("来源均为空应拒绝")
	}
	if _, err := s.CreateTicket(1, 0, 999, "alice", "note", nil, 1, "u", "ip", "ua"); err == nil {
		t.Fatal("来源归属组织不匹配应拒绝")
	}
}

// TestTicketLifecycle 工单闭环状态流转。
func TestTicketLifecycle(t *testing.T) {
	gdb := newTestDB(t)
	// 准备一个事件。
	ev := models.Event{OrgID: 1, EventType: "漏洞", Title: "xss", Severity: "high", Status: "pending"}
	if err := gdb.Create(&ev).Error; err != nil {
		t.Fatalf("create event: %v", err)
	}
	s := NewTriageService(gdb, nil, nil)

	tk, err := s.CreateTicket(1, ev.ID, 0, "", "先确认", nil, 1, "u", "ip", "ua")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if tk.EventID != ev.ID || tk.Status != "open" {
		t.Fatalf("ticket = %+v", tk)
	}

	// 派发。
	tk, err = s.AssignTicket(1, tk.ID, "alice", 1, 1, "u", "ip", "ua")
	if err != nil {
		t.Fatalf("assign: %v", err)
	}
	if tk.Assignee != "alice" {
		t.Fatalf("assignee = %s", tk.Assignee)
	}

	// 状态流转到 archived。
	for _, st := range []string{"dispatched", "fixing", "retest", "archived"} {
		tk, err = s.UpdateTicketStatus(1, tk.ID, st, tk.Version, 1, "u", "ip", "ua")
		if err != nil {
			t.Fatalf("status %s: %v", st, err)
		}
	}
	if tk.Status != "archived" {
		t.Fatalf("final status = %s", tk.Status)
	}
}

// TestTicketOptimisticLock 乐观锁冲突返回 409。
func TestTicketOptimisticLock(t *testing.T) {
	gdb := newTestDB(t)
	ev := models.Event{OrgID: 1, EventType: "漏洞", Title: "t", Severity: "high", Status: "pending"}
	if err := gdb.Create(&ev).Error; err != nil {
		t.Fatalf("create event: %v", err)
	}
	s := NewTriageService(gdb, nil, nil)
	tk, err := s.CreateTicket(1, ev.ID, 0, "", "", nil, 1, "u", "ip", "ua")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// 版本不匹配。
	if _, err := s.UpdateTicketStatus(1, tk.ID, "dispatched", 99, 1, "u", "ip", "ua"); err == nil {
		t.Fatal("版本冲突应拒绝")
	}
}

// TestTicketBatchStatus 批量状态流转 success/failed 分组。
func TestTicketBatchStatus(t *testing.T) {
	gdb := newTestDB(t)
	s := NewTriageService(gdb, nil, nil)
	var evIDs []int64
	for i := 0; i < 3; i++ {
		ev := models.Event{OrgID: 1, EventType: "漏洞", Title: "t", Severity: "high", Status: "pending"}
		if err := gdb.Create(&ev).Error; err != nil {
			t.Fatalf("create event: %v", err)
		}
		evIDs = append(evIDs, ev.ID)
	}
	var ids []int64
	for _, eid := range evIDs {
		tk, err := s.CreateTicket(1, eid, 0, "", "", nil, 1, "u", "ip", "ua")
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		ids = append(ids, tk.ID)
	}
	m, err := s.BatchUpdateTicketStatus(1, ids, "dispatched", 1, "u", "ip", "ua")
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(m["success"].([]int64)) != 3 {
		t.Fatalf("success = %v", m["success"])
	}
	if len(m["failed"].([]int64)) != 0 {
		t.Fatalf("failed = %v", m["failed"])
	}
}

// TestTicketDetail 详情含来源事件。
func TestTicketDetail(t *testing.T) {
	gdb := newTestDB(t)
	ev := models.Event{OrgID: 1, EventType: "漏洞", Title: "xss", Severity: "high", Status: "pending"}
	if err := gdb.Create(&ev).Error; err != nil {
		t.Fatalf("create event: %v", err)
	}
	s := NewTriageService(gdb, nil, nil)
	tk, err := s.CreateTicket(1, ev.ID, 0, "alice", "", nil, 1, "u", "ip", "ua")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	d, err := s.GetTicketDetail(1, tk.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if _, ok := d["event"]; !ok {
		t.Fatal("详情应含来源事件")
	}
}

// TestGetEventDetail 事件详情含关联工单。
func TestGetEventDetail(t *testing.T) {
	gdb := newTestDB(t)
	ev := models.Event{OrgID: 1, EventType: "漏洞", Title: "xss", Severity: "high", Status: "pending"}
	if err := gdb.Create(&ev).Error; err != nil {
		t.Fatalf("create event: %v", err)
	}
	s := NewTriageService(gdb, nil, nil)
	if _, err := s.CreateTicket(1, ev.ID, 0, "alice", "", nil, 1, "u", "ip", "ua"); err != nil {
		t.Fatalf("create: %v", err)
	}
	d, err := s.GetEventDetail(1, ev.ID)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	tickets, _ := d["tickets"].([]models.Ticket)
	if len(tickets) != 1 {
		t.Fatalf("tickets = %v", tickets)
	}
}
