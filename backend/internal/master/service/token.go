// Package service 实现 Master 业务逻辑层。
package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Fanc1er/Kate/backend/pkg/errs"
	"github.com/Fanc1er/Kate/backend/pkg/utils"
)

// TokenType 访问/刷新 token 类型。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims JWT 载荷。
type Claims struct {
	UserID    int64  `json:"uid"`
	Role      string `json:"role,omitempty"`
	TokenType string `json:"tt"`
	jwt.RegisteredClaims
}

// TokenPair 登录签发的双 token。
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"` // access token 剩余秒数
}

// TokenManager 负责 JWT 签发、校验与 refresh token jti 黑名单。
type TokenManager struct {
	secret      []byte
	accessTTL   time.Duration
	refreshTTL  time.Duration
	issuer      string
	mu          sync.Mutex
	blacklist   map[string]time.Time // jti -> 过期时间
	userIDByJTI map[string]int64     // jti -> user_id，用于改密时失效该用户全部 refresh
}

// NewTokenManager 构造 TokenManager。
func NewTokenManager(secret string, accessTTL, refreshTTL time.Duration) *TokenManager {
	if secret == "" {
		secret = utils.SHA256HexString(uuid.NewString())
	}
	return &TokenManager{
		secret:      []byte(secret),
		accessTTL:   accessTTL,
		refreshTTL:  refreshTTL,
		issuer:      "cinsight",
		blacklist:   make(map[string]time.Time),
		userIDByJTI: make(map[string]int64),
	}
}

// Issue 签发 access + refresh token 对。
func (m *TokenManager) Issue(userID int64) (*TokenPair, error) {
	now := time.Now()
	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()

	access, err := m.sign(&Claims{
		UserID:    userID,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        accessJTI,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	})
	if err != nil {
		return nil, err
	}
	refresh, err := m.sign(&Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        refreshJTI,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
		},
	})
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.userIDByJTI[refreshJTI] = userID
	m.mu.Unlock()
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(m.accessTTL.Seconds()),
	}, nil
}

// IssueWithRole 签发携带 role 的 access token（登录/刷新后使用）。
func (m *TokenManager) IssueWithRole(userID int64, role string) (string, error) {
	now := time.Now()
	return m.sign(&Claims{
		UserID:    userID,
		Role:      role,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	})
}

// Parse 解析并校验 JWT。
func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errs.New(errs.CodeTokenExpired, "")
		}
		return nil, errs.New(errs.CodeAuthFailed, "")
	}
	if !parsed.Valid {
		return nil, errs.New(errs.CodeAuthFailed, "")
	}
	if claims.TokenType != TokenTypeAccess {
		return nil, errs.New(errs.CodeAuthFailed, "")
	}
	m.mu.Lock()
	if exp, ok := m.blacklist[claims.ID]; ok && time.Now().Before(exp) {
		m.mu.Unlock()
		return nil, errs.New(errs.CodeAuthFailed, "凭证已失效")
	}
	m.mu.Unlock()
	return claims, nil
}

// ParseRefresh 解析 refresh token 并校验类型与黑名单。
func (m *TokenManager) ParseRefresh(token string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errs.New(errs.CodeTokenExpired, "")
		}
		return nil, errs.New(errs.CodeAuthFailed, "")
	}
	if !parsed.Valid || claims.TokenType != TokenTypeRefresh {
		return nil, errs.New(errs.CodeAuthFailed, "")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if exp, ok := m.blacklist[claims.ID]; ok && time.Now().Before(exp) {
		return nil, errs.New(errs.CodeAuthFailed, "凭证已失效")
	}
	return claims, nil
}

// RevokeJTI 拉黑单个 jti。
func (m *TokenManager) RevokeJTI(jti string, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blacklist[jti] = time.Now().Add(ttl)
}

// RevokeAllUserRefresh 撤销某用户全部 refresh token（改密/注销后调用）。
func (m *TokenManager) RevokeAllUserRefresh(userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for jti, uid := range m.userIDByJTI {
		if uid == userID {
			m.blacklist[jti] = time.Now().Add(m.refreshTTL)
		}
	}
}

func (m *TokenManager) sign(claims *Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}
