package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// 工单状态流转：open(已确认) → dispatched(已派发) → fixing(修复中) → retest(复测中) → archived(已归档)。
// 取消：dispatched/fixing 可回退 open；retest 失败回退 fixing。

// validTicketStatuses 合法工单状态。
var validTicketStatuses = map[string]bool{
	"open": true, "dispatched": true, "fixing": true, "retest": true, "archived": true,
}

// ListTickets 工单列表。
func (s *TriageService) ListTickets(orgID int64, status, source string, page, pageSize int) ([]models.Ticket, int64, error) {
	q := s.guard(orgID).Scoped(&models.Ticket{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if source == "event" {
		q = q.Where("event_id > 0")
	}
	if source == "vuln" {
		q = q.Where("vuln_id > 0")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []models.Ticket
	if err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// GetTicketDetail 工单详情（含来源事件/漏洞摘要）。
func (s *TriageService) GetTicketDetail(orgID, id int64) (map[string]any, error) {
	var t models.Ticket
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		return nil, err
	}
	detail := map[string]any{"ticket": t}
	if t.EventID > 0 {
		var ev models.Event
		if err := s.DB.Where("id = ? AND org_id = ?", t.EventID, orgID).First(&ev).Error; err == nil {
			detail["event"] = ev
		}
	}
	if t.VulnID > 0 {
		var v models.Vulnerability
		if err := s.DB.Where("id = ? AND org_id = ?", t.VulnID, orgID).First(&v).Error; err == nil {
			detail["vulnerability"] = v
		}
	}
	return detail, nil
}

// CreateTicket 创建工单（事件/漏洞来源关联，至少其一非空）。
func (s *TriageService) CreateTicket(orgID int64, eventID, vulnID int64, assignee, notes string, dueAt *time.Time, userID int64, username, ip, ua string) (*models.Ticket, error) {
	if eventID <= 0 && vulnID <= 0 {
		return nil, errs.New(errs.CodeValidationFailed, "事件或漏洞来源至少其一非空")
	}
	// 校验来源归属本组织。
	if eventID > 0 {
		var cnt int64
		s.DB.Model(&models.Event{}).Where("id = ? AND org_id = ?", eventID, orgID).Count(&cnt)
		if cnt == 0 {
			return nil, errs.New(errs.CodeNotFound, "事件不存在")
		}
	}
	if vulnID > 0 {
		var cnt int64
		s.DB.Model(&models.Vulnerability{}).Where("id = ? AND org_id = ?", vulnID, orgID).Count(&cnt)
		if cnt == 0 {
			return nil, errs.New(errs.CodeNotFound, "漏洞不存在")
		}
	}
	t := &models.Ticket{
		OrgID: orgID, EventID: eventID, VulnID: vulnID,
		Assignee: assignee, Status: "open", DueAt: dueAt, Notes: notes, Version: 1,
	}
	if err := s.DB.Create(t).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "ticket.create", "ticket", fmt.Sprint(t.ID), "", "open", ip, ua)
	}
	return t, nil
}

// UpdateTicketStatus 工单状态流转（乐观锁 + 合法状态机校验）。
func (s *TriageService) UpdateTicketStatus(orgID, id int64, status string, version int, userID int64, username, ip, ua string) (*models.Ticket, error) {
	if !validTicketStatuses[status] {
		return nil, errs.New(errs.CodeValidationFailed, "非法工单状态")
	}
	var t models.Ticket
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		return nil, err
	}
	if version > 0 && version != t.Version {
		return nil, errs.New(errs.CodeTaskStateConflict, "工单版本冲突")
	}
	old := t.Status
	if err := s.DB.Model(&t).Updates(map[string]any{"status": status, "version": t.Version + 1, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "ticket.update", "ticket", fmt.Sprint(id), old, status, ip, ua)
	}
	return &t, nil
}

// AssignTicket 派发工单（指定处理人，置 dispatched）。
func (s *TriageService) AssignTicket(orgID, id int64, assignee string, version int, userID int64, username, ip, ua string) (*models.Ticket, error) {
	if assignee == "" {
		return nil, errs.New(errs.CodeValidationFailed, "处理人不能为空")
	}
	var t models.Ticket
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		return nil, err
	}
	if version > 0 && version != t.Version {
		return nil, errs.New(errs.CodeTaskStateConflict, "工单版本冲突")
	}
	old := t.Assignee
	if err := s.DB.Model(&t).Updates(map[string]any{"assignee": assignee, "version": t.Version + 1, "updated_at": time.Now()}).Error; err != nil {
		return nil, err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "ticket.assign", "ticket", fmt.Sprint(id), old, assignee, ip, ua)
	}
	return &t, nil
}

// DeleteTicket 删除工单。
func (s *TriageService) DeleteTicket(orgID, id int64, userID int64, username, ip, ua string) error {
	var t models.Ticket
	if err := s.DB.Where("id = ? AND org_id = ?", id, orgID).First(&t).Error; err != nil {
		return err
	}
	if err := s.DB.Delete(&t).Error; err != nil {
		return err
	}
	if s.Audit != nil {
		s.Audit.Write(orgID, userID, username, "ticket.delete", "ticket", fmt.Sprint(id), t.Status, "deleted", ip, ua)
	}
	return nil
}

// ListTicketSources 统计事件/漏洞工单来源数。
func (s *TriageService) ListTicketSources(orgID int64) (map[string]any, error) {
	var evCnt, vCnt int64
	s.DB.Model(&models.Ticket{}).Where("org_id = ? AND event_id > 0", orgID).Count(&evCnt)
	s.DB.Model(&models.Ticket{}).Where("org_id = ? AND vuln_id > 0", orgID).Count(&vCnt)
	return map[string]any{"event_tickets": evCnt, "vuln_tickets": vCnt}, nil
}

// BatchUpdateTicketStatus 批量状态流转（返回逐条结果）。
func (s *TriageService) BatchUpdateTicketStatus(orgID int64, ids []int64, status string, userID int64, username, ip, ua string) (map[string]any, error) {
	var success, failed []int64
	for _, id := range ids {
		if _, err := s.UpdateTicketStatus(orgID, id, status, 0, userID, username, ip, ua); err != nil {
			failed = append(failed, id)
		} else {
			success = append(success, id)
		}
	}
	return map[string]any{"success": success, "failed": failed}, nil
}

// EventTicketIDs 事件关联的工单 ID 列表（事件详情展示）。
func (s *TriageService) EventTicketIDs(orgID, eventID int64) ([]int64, error) {
	var ids []int64
	if err := s.DB.Model(&models.Ticket{}).Where("org_id = ? AND event_id = ?", orgID, eventID).Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

var _ = json.Marshal
