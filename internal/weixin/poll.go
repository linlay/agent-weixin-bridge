package weixin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"agent-weixin-bridge/internal/diag"
	"agent-weixin-bridge/internal/state"
)

type InboundHandler interface {
	HandleInbound(context.Context, state.Credential, Message) error
}

type PollManager struct {
	client  *Client
	store   *state.FileStore
	service InboundHandler
	botType string

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewPollManager(client *Client, store *state.FileStore, service InboundHandler, botType string) *PollManager {
	return &PollManager{
		client:  client,
		store:   store,
		service: service,
		botType: botType,
	}
}

func (m *PollManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		diag.Info("poll.start.noop", "reason", "already_running")
		return nil
	}
	cred, ok, err := m.store.LoadCredential()
	if err != nil {
		diag.Error("poll.start.load_credential_failed", "error", err)
		return err
	}
	if !ok || strings.TrimSpace(cred.BotToken) == "" {
		err := fmt.Errorf("weixin account not configured")
		diag.Warn("poll.start.skipped", "reason", "missing_credential")
		return err
	}
	diag.Info("poll.start.attempt", "account_id", cred.AccountID, "base_url", cred.BaseURL)
	loopCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.running = true
	go m.loop(loopCtx, cred)
	if err := m.store.SaveStatus(state.AccountStatus{
		Configured: true,
		Polling:    true,
		AccountID:  cred.AccountID,
	}); err != nil {
		diag.Error("state.status.save_failed", "reason", "poll_start", "error", err)
		return err
	}
	diag.Info("state.status.updated", "reason", "poll_start", "polling_after", true, "account_id", cred.AccountID)
	diag.Info("poll.start.started", "account_id", cred.AccountID)
	return nil
}

func (m *PollManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	wasRunning := m.running
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.running = false
	status, _ := m.store.LoadStatus()
	previousPolling := status.Polling
	status.Polling = false
	if err := m.store.SaveStatus(status); err != nil {
		diag.Error("state.status.save_failed", "reason", "poll_stop", "error", err)
	} else {
		diag.Info("state.status.updated", "reason", "poll_stop", "polling_before", previousPolling, "polling_after", status.Polling, "account_id", status.AccountID)
	}
	diag.Info("poll.stop.completed", "was_running", wasRunning)
}

func (m *PollManager) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *PollManager) loop(ctx context.Context, cred state.Credential) {
	client := m.client.ForBaseURL(cred.BaseURL)
	cursor, err := m.store.LoadSyncCursor()
	if err != nil {
		diag.Error("poll.cursor.load_failed", "error", err)
	}
	diag.Info("poll.loop.started", "account_id", cred.AccountID, "base_url", cred.BaseURL, "cursor_present", strings.TrimSpace(cursor) != "")
	timeout := 35 * time.Second
	for {
		select {
		case <-ctx.Done():
			diag.Info("poll.loop.stopped", "account_id", cred.AccountID, "reason", ctx.Err())
			return
		default:
		}

		diag.Debug("poll.getupdates.request",
			"account_id", cred.AccountID,
			"cursor_present", strings.TrimSpace(cursor) != "",
			"timeout_ms", timeout.Milliseconds(),
		)
		reqCtx, cancel := context.WithTimeout(ctx, timeout+5*time.Second)
		resp, err := client.GetUpdates(reqCtx, cred.BotToken, cursor)
		cancel()
		if err != nil {
			m.recordError(err)
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.LongPollingTimeoutMs > 0 {
			timeout = time.Duration(resp.LongPollingTimeoutMs) * time.Millisecond
		}
		diag.Debug("poll.getupdates.response",
			"account_id", cred.AccountID,
			"message_count", len(resp.Messages),
			"ret", resp.Ret,
			"errcode", resp.ErrCode,
			"errmsg", resp.ErrMessage,
			"cursor_changed", resp.GetUpdatesBuf != "" && resp.GetUpdatesBuf != cursor,
			"next_timeout_ms", timeout.Milliseconds(),
		)
		if resp.GetUpdatesBuf != "" && resp.GetUpdatesBuf != cursor {
			cursor = resp.GetUpdatesBuf
			if err := m.store.SaveSyncCursor(cursor); err != nil {
				diag.Error("poll.cursor.save_failed", "error", err)
			} else {
				diag.Debug("poll.cursor.saved", "cursor_preview", diag.PreviewText(cursor, 48))
			}
		}
		if resp.Ret != 0 || resp.ErrCode != 0 {
			m.recordError(fmt.Errorf("getupdates ret=%d errcode=%d errmsg=%s", resp.Ret, resp.ErrCode, resp.ErrMessage))
			time.Sleep(2 * time.Second)
			continue
		}
		for _, msg := range resp.Messages {
			text, hasText := previewMessageText(msg)
			diag.Info("poll.inbound.dispatch",
				"from_user_id", msg.FromUserID,
				"context_token", diag.PreviewText(msg.ContextToken, 32),
				"message_type", msg.MessageType,
				"text_len", len([]rune(text)),
				"text_preview", diag.PreviewText(text, 160),
				"has_text", hasText,
				"create_time_ms", msg.CreateTimeMs,
			)
			if err := m.service.HandleInbound(ctx, cred, msg); err != nil {
				m.recordError(err)
			} else {
				diag.Info("poll.inbound.handled", "from_user_id", msg.FromUserID, "context_token", diag.PreviewText(msg.ContextToken, 32))
			}
		}
	}
}

func (m *PollManager) recordError(err error) {
	diag.Error("poll.error", "error", err)
	status, _ := m.store.LoadStatus()
	previousError := status.LastError
	status.Polling = true
	status.LastError = err.Error()
	if saveErr := m.store.SaveStatus(status); saveErr != nil {
		diag.Error("state.status.save_failed", "reason", "poll_error", "error", saveErr)
		return
	}
	diag.Info("state.status.updated",
		"reason", "poll_error",
		"polling_after", status.Polling,
		"last_error_before", diag.PreviewText(previousError, 160),
		"last_error_after", diag.PreviewText(status.LastError, 160),
	)
}

func previewMessageText(msg Message) (string, bool) {
	for _, item := range msg.ItemList {
		if item.Type == 1 && item.TextItem != nil && strings.TrimSpace(item.TextItem.Text) != "" {
			return strings.TrimSpace(item.TextItem.Text), true
		}
	}
	return "", false
}
