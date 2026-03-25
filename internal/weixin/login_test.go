package weixin

import (
	"testing"
	"time"
)

func TestLoginServiceGetSessionReturnsValidSession(t *testing.T) {
	service := &LoginService{
		sessions: map[string]LoginSession{
			"session-1": {
				SessionKey: "session-1",
				QRCodeURL:  "data:image/png;base64,AAAA",
				ExpiresAt:  time.Now().Add(time.Minute),
			},
		},
	}

	session, ok := service.GetSession("session-1")
	if !ok {
		t.Fatalf("GetSession() ok = false, want true")
	}
	if session.SessionKey != "session-1" {
		t.Fatalf("SessionKey = %q, want %q", session.SessionKey, "session-1")
	}
}

func TestLoginServiceGetSessionDeletesExpiredSession(t *testing.T) {
	service := &LoginService{
		sessions: map[string]LoginSession{
			"expired": {
				SessionKey: "expired",
				ExpiresAt:  time.Now().Add(-time.Minute),
			},
		},
	}

	_, ok := service.GetSession("expired")
	if ok {
		t.Fatalf("GetSession() ok = true, want false")
	}
	if _, exists := service.sessions["expired"]; exists {
		t.Fatalf("expired session was not deleted")
	}
}
