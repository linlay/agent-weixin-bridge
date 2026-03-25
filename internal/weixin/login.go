package weixin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"agent-weixin-bridge/internal/state"
)

type LoginSession struct {
	SessionKey string    `json:"sessionKey"`
	QRCodeURL  string    `json:"qrCodeUrl"`
	ExpiresAt  time.Time `json:"expiresAt"`
	QRCode     string
}

type LoginStatus struct {
	SessionKey string `json:"sessionKey"`
	Status     string `json:"status"`
	Connected  bool   `json:"connected"`
	AccountID  string `json:"accountId,omitempty"`
	Message    string `json:"message"`
}

type LoginService struct {
	client  *Client
	store   *state.FileStore
	botType string

	mu       sync.Mutex
	sessions map[string]LoginSession
}

func NewLoginService(client *Client, store *state.FileStore, botType string) *LoginService {
	return &LoginService{
		client:   client,
		store:    store,
		botType:  botType,
		sessions: map[string]LoginSession{},
	}
}

func (s *LoginService) Start(ctx context.Context) (LoginSession, error) {
	resp, err := s.client.StartQRLogin(ctx, s.botType)
	if err != nil {
		return LoginSession{}, err
	}
	session := LoginSession{
		SessionKey: newSessionKey(),
		QRCodeURL:  resp.QRCodeImageURL,
		ExpiresAt:  time.Now().Add(5 * time.Minute),
		QRCode:     resp.QRCode,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.SessionKey] = session
	return session, nil
}

func (s *LoginService) Check(ctx context.Context, sessionKey string) (LoginStatus, error) {
	s.mu.Lock()
	session, ok := s.sessions[sessionKey]
	if !ok {
		s.mu.Unlock()
		return LoginStatus{}, fmt.Errorf("login session not found")
	}
	if time.Now().After(session.ExpiresAt) {
		delete(s.sessions, sessionKey)
		s.mu.Unlock()
		return LoginStatus{
			SessionKey: sessionKey,
			Status:     "expired",
			Message:    "二维码已过期，请重新生成。",
		}, nil
	}
	s.mu.Unlock()

	resp, err := s.client.GetQRLoginStatus(ctx, session.QRCode)
	if err != nil {
		return LoginStatus{}, err
	}
	if resp.Status == "confirmed" && resp.BotToken != "" && resp.ILinkBotID != "" {
		cred := state.Credential{
			BotToken:  resp.BotToken,
			BaseURL:   resp.BaseURL,
			AccountID: resp.ILinkBotID,
			UserID:    resp.UserID,
			SavedAt:   time.Now(),
		}
		if err := s.store.SaveCredential(cred); err != nil {
			return LoginStatus{}, err
		}
		_ = s.store.SaveStatus(state.AccountStatus{
			Configured: true,
			AccountID:  cred.AccountID,
		})
		s.mu.Lock()
		delete(s.sessions, sessionKey)
		s.mu.Unlock()
		return LoginStatus{
			SessionKey: sessionKey,
			Status:     "confirmed",
			Connected:  true,
			AccountID:  resp.ILinkBotID,
			Message:    "与微信连接成功。",
		}, nil
	}
	return LoginStatus{
		SessionKey: sessionKey,
		Status:     resp.Status,
		Connected:  false,
		Message:    loginStatusMessage(resp.Status),
	}, nil
}

func (s *LoginService) GetSession(sessionKey string) (LoginSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionKey]
	if !ok {
		return LoginSession{}, false
	}
	if time.Now().After(session.ExpiresAt) {
		delete(s.sessions, sessionKey)
		return LoginSession{}, false
	}
	return session, true
}

func loginStatusMessage(status string) string {
	switch status {
	case "wait":
		return "等待扫码。"
	case "scaned":
		return "已扫码，请在微信确认。"
	case "expired":
		return "二维码已过期，请重新生成。"
	default:
		return "等待连接结果。"
	}
}

func newSessionKey() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf[:])
}
