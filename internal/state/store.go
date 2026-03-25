package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Credential struct {
	BotToken  string    `json:"botToken"`
	BaseURL   string    `json:"baseURL"`
	AccountID string    `json:"accountId"`
	UserID    string    `json:"userId,omitempty"`
	SavedAt   time.Time `json:"savedAt"`
}

type UserContext struct {
	ChatID       string    `json:"chatId"`
	ContextToken string    `json:"contextToken,omitempty"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
}

type AccountStatus struct {
	Configured     bool      `json:"configured"`
	Polling        bool      `json:"polling"`
	AccountID      string    `json:"accountId,omitempty"`
	LastInboundAt  time.Time `json:"lastInboundAt,omitempty"`
	LastOutboundAt time.Time `json:"lastOutboundAt,omitempty"`
	LastError      string    `json:"lastError,omitempty"`
}

type FileStore struct {
	root string
	mu   sync.RWMutex
}

func NewFileStore(root string) (*FileStore, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir state dir: %w", err)
	}
	return &FileStore{root: root}, nil
}

func (s *FileStore) SaveCredential(cred Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON("credential.json", cred)
}

func (s *FileStore) LoadCredential() (Credential, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var cred Credential
	ok, err := s.readJSON("credential.json", &cred)
	return cred, ok, err
}

func (s *FileStore) SaveSyncCursor(cursor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON("sync_cursor.json", struct {
		Buffer string `json:"buffer"`
	}{Buffer: cursor})
}

func (s *FileStore) LoadSyncCursor() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var payload struct {
		Buffer string `json:"buffer"`
	}
	ok, err := s.readJSON("sync_cursor.json", &payload)
	if err != nil || !ok {
		return "", err
	}
	return payload.Buffer, nil
}

func (s *FileStore) SaveUserContext(userID string, user UserContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	users, err := s.readUsersLocked()
	if err != nil {
		return err
	}
	users[userID] = user
	return s.writeJSON("users.json", users)
}

func (s *FileStore) LoadUserContext(userID string) (UserContext, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users := map[string]UserContext{}
	ok, err := s.readJSON("users.json", &users)
	if err != nil || !ok {
		return UserContext{}, false, err
	}
	user, found := users[userID]
	return user, found, nil
}

func (s *FileStore) SaveStatus(status AccountStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeJSON("status.json", status)
}

func (s *FileStore) LoadStatus() (AccountStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var status AccountStatus
	ok, err := s.readJSON("status.json", &status)
	if err != nil {
		return AccountStatus{}, err
	}
	if !ok {
		return AccountStatus{}, nil
	}
	return status, nil
}

func (s *FileStore) readUsersLocked() (map[string]UserContext, error) {
	users := map[string]UserContext{}
	_, err := s.readJSON("users.json", &users)
	return users, err
}

func (s *FileStore) path(name string) string {
	return filepath.Join(s.root, name)
}

func (s *FileStore) readJSON(name string, target any) (bool, error) {
	path := s.path(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", name, err)
	}
	return true, nil
}

func (s *FileStore) writeJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	if err := os.WriteFile(s.path(name), data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}
