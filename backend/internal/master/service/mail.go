package service

import (
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
)

// MailService 系统级邮件发送（验证码/邀请邮件），与业务通知渠道相互独立。
type MailService struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	CodeTTL  time.Duration
}

// NewMailService 构造 MailService；未配置 SMTP 时返回 nil（发送降级为空操作）。
func NewMailService(host string, port int, user, password, from string, codeTTL time.Duration) *MailService {
	if host == "" {
		return nil
	}
	return &MailService{Host: host, Port: port, User: user, Password: password, From: from, CodeTTL: codeTTL}
}

// Send 发送一封邮件。
func (m *MailService) Send(to, subject, body string) error {
	if m == nil {
		return errs.New(errs.CodeNotifyFailed, "系统邮件未配置")
	}
	from := m.From
	if from == "" {
		from = m.User
	}
	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		from, to, subject, body,
	))
	var auth smtp.Auth
	if m.User != "" {
		auth = smtp.PlainAuth("", m.User, m.Password, m.Host)
	}
	if err := smtp.SendMail(addr, auth, from, []string{to}, msg); err != nil {
		return err
	}
	return nil
}

// EmailMask 用于日志脱敏。
func EmailMask(e string) string {
	at := strings.Index(e, "@")
	if at <= 0 {
		return "***"
	}
	return e[:1] + "***" + e[at:]
}
