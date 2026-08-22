package service

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/Fanc1er/Kate/backend/internal/master/models"
	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/rbac"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// LoginLockConfig 登录锁定阈值。
const (
	LoginMaxAttempts = 5
	LoginLockMinutes = 15
)

// AuthService 认证服务。
type AuthService struct {
	DB     *gorm.DB
	Tokens *TokenManager
	Mail   *MailService

	mu       sync.Mutex
	attempts map[string]*loginAttempt // username -> 失败计数
	lockout  map[string]time.Time     // username -> 解锁时间
	ipLimit  map[string]*ipWindow     // ip -> 窗口计数
	mailCodes map[string]*mailCode    // email -> 验证码
}

type loginAttempt struct {
	Count    int
	LastFail time.Time
}

type ipWindow struct {
	Count    int
	WindowAt time.Time
}

type mailCode struct {
	Code      string
	ExpiresAt time.Time
}

// NewAuthService 构造 AuthService。
func NewAuthService(db *gorm.DB, tokens *TokenManager, mail *MailService) *AuthService {
	return &AuthService{
		DB:        db,
		Tokens:    tokens,
		Mail:      mail,
		attempts:  make(map[string]*loginAttempt),
		lockout:   make(map[string]time.Time),
		ipLimit:   make(map[string]*ipWindow),
		mailCodes: make(map[string]*mailCode),
	}
}

// LoginResult 登录成功返回。
type LoginResult struct {
	TokenPair
	User *UserDTO `json:"user"`
}

// UserDTO 当前用户信息。
type UserDTO struct {
	ID          int64    `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	AvatarURL   string   `json:"avatar_url"`
	Status      string   `json:"status"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// Login 用户名密码登录。签发携带 role 的 access token。
func (s *AuthService) Login(username, password, ip string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errs.New(errs.CodeValidationFailed, "用户名和密码不能为空")
	}
	// 登录接口限流：每 IP 5 次/min。
	s.mu.Lock()
	if w, ok := s.ipLimit[ip]; ok {
		if time.Since(w.WindowAt) < time.Minute {
			if w.Count >= LoginMaxAttempts {
				s.mu.Unlock()
				return nil, errs.New(errs.CodeValidationFailed, "登录尝试过于频繁，请稍后再试")
			}
			w.Count++
		} else {
			w.Count, w.WindowAt = 1, time.Now()
		}
	} else {
		s.ipLimit[ip] = &ipWindow{Count: 1, WindowAt: time.Now()}
	}
	// 账户锁定检查。
	if unlock, ok := s.lockout[username]; ok && time.Now().Before(unlock) {
		s.mu.Unlock()
		return nil, errs.New(errs.CodeAccountLocked, "")
	}
	s.mu.Unlock()

	var user models.User
	if err := s.DB.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.recordFailure(username)
			return nil, errs.New(errs.CodeAuthFailed, "用户名或密码错误")
		}
		return nil, err
	}
	if user.Status == models.StatusDisabled {
		return nil, errs.New(errs.CodeUserDisabled, "")
	}
	if !utils.CheckPassword(user.Password, password) {
		s.recordFailure(username)
		return nil, errs.New(errs.CodeAuthFailed, "用户名或密码错误")
	}
	s.clearFailure(username)

	pair, err := s.Tokens.Issue(user.ID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	s.DB.Model(&user).Update("last_login_at", now)

	access, err := s.Tokens.IssueWithRole(user.ID, user.Role)
	if err != nil {
		return nil, err
	}
	pair.AccessToken = access
	return &LoginResult{TokenPair: *pair, User: s.buildUserDTO(&user)}, nil
}

// Me 返回当前用户信息（仅依赖 JWT）。
func (s *AuthService) Me(userID int64) (*UserDTO, error) {
	var user models.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	return s.buildUserDTO(&user), nil
}

// Refresh 用 refresh token 换发新 access token。
func (s *AuthService) Refresh(refreshToken string) (*TokenPair, error) {
	claims, err := s.Tokens.ParseRefresh(refreshToken)
	if err != nil {
		return nil, err
	}
	// 刷新后旧 refresh 失效。
	s.Tokens.RevokeJTI(claims.ID, s.Tokens.refreshTTL)
	return s.Tokens.Issue(claims.UserID)
}

// Logout 登出：拉黑当前 refresh token。
func (s *AuthService) Logout(refreshToken string) {
	if refreshToken != "" {
		if claims, err := s.Tokens.ParseRefresh(refreshToken); err == nil {
			s.Tokens.RevokeJTI(claims.ID, s.Tokens.refreshTTL)
		}
	}
}

// ChangePassword 登录态改密：校验旧密码，改后失效全部 refresh token。
func (s *AuthService) ChangePassword(userID int64, oldPwd, newPwd string) error {
	var user models.User
	if err := s.DB.First(&user, userID).Error; err != nil {
		return err
	}
	if !utils.CheckPassword(user.Password, oldPwd) {
		return errs.New(errs.CodeAuthFailed, "原密码错误")
	}
	if err := validatePassword(newPwd); err != nil {
		return err
	}
	if utils.CheckPassword(user.Password, newPwd) {
		return errs.New(errs.CodeValidationFailed, "新密码不能与原密码相同")
	}
	hash, err := utils.HashPassword(newPwd)
	if err != nil {
		return err
	}
	if err := s.DB.Model(&user).Update("password", hash).Error; err != nil {
		return err
	}
	s.Tokens.RevokeAllUserRefresh(userID)
	return nil
}

// ForgotPassword 忘记密码：生成一次性邮件验证码。
func (s *AuthService) ForgotPassword(email string) error {
	if email == "" {
		return errs.New(errs.CodeValidationFailed, "邮箱不能为空")
	}
	var user models.User
	if err := s.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// 不泄露账号是否存在，统一返回成功。
		return nil
	}
	if s.Mail == nil {
		return errs.New(errs.CodeNotifyFailed, "系统邮件未配置，请联系管理员")
	}
	code := randomCode(6)
	s.mu.Lock()
	s.mailCodes[email] = &mailCode{Code: code, ExpiresAt: time.Now().Add(s.Mail.CodeTTL)}
	s.mu.Unlock()
	if err := s.Mail.Send(email, "CInsight 密码重置验证码", fmt.Sprintf("您的验证码是 %s，5 分钟内有效。", code)); err != nil {
		return errs.Wrap(errs.CodeNotifyFailed, err)
	}
	return nil
}

// ResetPassword 重置密码：校验验证码 + 新密码。
func (s *AuthService) ResetPassword(email, code, newPwd string) error {
	s.mu.Lock()
	rec, ok := s.mailCodes[email]
	if ok {
		delete(s.mailCodes, email)
	}
	s.mu.Unlock()
	if !ok || time.Now().After(rec.ExpiresAt) || rec.Code != code {
		return errs.New(errs.CodeValidationFailed, "验证码无效或已过期")
	}
	if err := validatePassword(newPwd); err != nil {
		return err
	}
	hash, err := utils.HashPassword(newPwd)
	if err != nil {
		return err
	}
	if err := s.DB.Model(&models.User{}).Where("email = ?", email).Update("password", hash).Error; err != nil {
		return err
	}
	var users []models.User
	s.DB.Where("email = ?", email).Find(&users)
	for _, u := range users {
		s.Tokens.RevokeAllUserRefresh(u.ID)
	}
	return nil
}

func (s *AuthService) buildUserDTO(u *models.User) *UserDTO {
	return &UserDTO{
		ID:          u.ID,
		Username:    u.Username,
		Email:       u.Email,
		Phone:       u.Phone,
		AvatarURL:   u.AvatarURL,
		Status:      u.Status,
		Role:        u.Role,
		Permissions: rbac.PermissionsOf(u.Role),
	}
}

func (s *AuthService) recordFailure(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.attempts[username]
	if rec == nil {
		rec = &loginAttempt{}
		s.attempts[username] = rec
	}
	rec.Count++
	rec.LastFail = time.Now()
	if rec.Count >= LoginMaxAttempts {
		s.lockout[username] = time.Now().Add(LoginLockMinutes * time.Minute)
		delete(s.attempts, username)
	}
}

func (s *AuthService) clearFailure(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.attempts, username)
}

func validatePassword(pwd string) error {
	if len(pwd) < 12 {
		return errs.New(errs.CodeValidationFailed, "密码长度至少 12 位")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range pwd {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	if !(hasUpper && hasLower && hasDigit && hasSpecial) {
		return errs.New(errs.CodeValidationFailed, "密码需包含大小写字母、数字与特殊字符")
	}
	return nil
}

func randomCode(n int) string {
	const digits = "0123456789"
	b := make([]byte, n)
	for i := range b {
		v, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		b[i] = digits[v.Int64()]
	}
	return string(b)
}

var _ = uuid.NewString
