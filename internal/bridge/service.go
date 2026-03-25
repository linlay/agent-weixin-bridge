package bridge

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"agent-weixin-bridge/internal/diag"
	"agent-weixin-bridge/internal/runner"
	"agent-weixin-bridge/internal/state"
	"agent-weixin-bridge/internal/weixin"
)

const (
	flushRuneThreshold = 200
	flushInterval      = 2 * time.Second
	typingInterval     = 5 * time.Second
)

type Service struct {
	wx     *weixin.Client
	runner *runner.Client
	store  *state.FileStore

	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func NewService(wx *weixin.Client, runnerClient *runner.Client, store *state.FileStore) *Service {
	return &Service{
		wx:     wx,
		runner: runnerClient,
		store:  store,
		locks:  map[string]*sync.Mutex{},
	}
}

func (s *Service) HandleInbound(ctx context.Context, cred state.Credential, msg weixin.Message) error {
	if msg.MessageType != 0 && msg.MessageType != 1 {
		diag.Info("bridge.inbound.skipped", "reason", "unsupported_message_type", "message_type", msg.MessageType, "from_user_id", msg.FromUserID)
		return nil
	}
	userID := strings.TrimSpace(msg.FromUserID)
	if userID == "" {
		diag.Warn("bridge.inbound.skipped", "reason", "missing_from_user")
		return nil
	}
	text, isText := extractText(msg)
	diag.Info("bridge.inbound.received",
		"account_id", cred.AccountID,
		"from_user_id", userID,
		"context_token", diag.PreviewText(msg.ContextToken, 32),
		"message_type", msg.MessageType,
		"create_time_ms", msg.CreateTimeMs,
		"text_len", utf8.RuneCountInString(text),
		"text_preview", diag.PreviewText(text, 160),
		"is_text", isText,
	)
	lock := s.userLock(userID)
	lock.Lock()
	defer lock.Unlock()

	s.saveStatus("bridge_inbound_received", func(status *state.AccountStatus) {
		status.Configured = true
		status.AccountID = cred.AccountID
		status.Polling = true
		status.LastInboundAt = time.Now()
	}, "from_user_id", userID)
	wxClient := s.wx.ForBaseURL(cred.BaseURL)

	if !isText {
		diag.Info("bridge.inbound.non_text", "from_user_id", userID, "context_token", diag.PreviewText(msg.ContextToken, 32))
		return s.sendText(ctx, wxClient, cred, userID, msg.ContextToken, "当前仅支持文本消息。")
	}

	userContext, err := s.loadOrCreateUserContext(userID, msg.ContextToken)
	if err != nil {
		diag.Error("bridge.user_context.failed", "from_user_id", userID, "error", err)
		return err
	}
	contextToken := s.resolveContextToken(userID, msg.ContextToken)
	diag.Info("bridge.runner.dispatch",
		"from_user_id", userID,
		"chat_id", userContext.ChatID,
		"context_token", diag.PreviewText(contextToken, 32),
		"text_len", utf8.RuneCountInString(text),
		"text_preview", diag.PreviewText(text, 160),
	)

	typingCtx, stopTyping := context.WithCancel(context.Background())
	defer stopTyping()
	go s.runTypingLoop(typingCtx, wxClient, cred, userID, contextToken)

	flusher := newTextFlusher(func(chunk string) error {
		return s.sendText(ctx, wxClient, cred, userID, contextToken, chunk)
	})

	var snapshotText string
	var sawRichEvent bool
	var terminalResponseSent bool

	err = s.runner.StreamQuery(ctx, userContext.ChatID, text, func(event runner.StreamEvent) error {
		diag.Debug("bridge.runner.event",
			"from_user_id", userID,
			"chat_id", userContext.ChatID,
			"type", event.Type,
			"run_id", event.RunID,
			"delta_len", utf8.RuneCountInString(event.Delta),
			"text_len", utf8.RuneCountInString(event.Text),
			"error_message", event.ErrorMessage,
		)
		switch event.Type {
		case "content.delta":
			return flusher.Add(event.Delta)
		case "content.snapshot":
			snapshotText = strings.TrimSpace(event.Text)
			return nil
		case "run.complete":
			return flusher.Flush()
		case "run.cancel":
			return flusher.Flush()
		case "run.error":
			if err := flusher.Flush(); err != nil {
				return err
			}
			message := "微信桥接到 runner 失败。"
			if event.ErrorMessage != "" {
				message = "微信桥接到 runner 失败：" + event.ErrorMessage
			}
			terminalResponseSent = true
			if err := s.sendText(ctx, wxClient, cred, userID, contextToken, message); err != nil {
				return err
			}
			s.recordStatusError(cred, message)
			return nil
		default:
			if isRichRunnerEvent(event.Type) {
				sawRichEvent = true
			}
			return nil
		}
	})
	stopTyping()
	if cancelErr := s.stopTyping(context.Background(), wxClient, cred, userID, contextToken); cancelErr != nil {
		diag.Warn("bridge.typing.stop_failed", "from_user_id", userID, "error", cancelErr)
	}
	if err != nil {
		diag.Error("bridge.runner.failed", "from_user_id", userID, "chat_id", userContext.ChatID, "error", err)
		sendErr := s.sendText(ctx, wxClient, cred, userID, contextToken, "请求处理失败，请稍后重试。")
		s.recordStatusError(cred, err.Error())
		if sendErr != nil {
			return fmt.Errorf("runner error: %w; send fallback: %v", err, sendErr)
		}
		return err
	}
	if terminalResponseSent {
		diag.Info("bridge.runner.completed", "from_user_id", userID, "chat_id", userContext.ChatID, "result", "terminal_response_sent")
		return nil
	}
	if !flusher.SentAny() && snapshotText != "" {
		diag.Info("bridge.runner.snapshot_fallback", "from_user_id", userID, "chat_id", userContext.ChatID, "text_preview", diag.PreviewText(snapshotText, 160))
		return s.sendText(ctx, wxClient, cred, userID, contextToken, snapshotText)
	}
	if !flusher.SentAny() && sawRichEvent {
		diag.Info("bridge.runner.rich_event_fallback", "from_user_id", userID, "chat_id", userContext.ChatID)
		return s.sendText(ctx, wxClient, cred, userID, contextToken, "当前该智能体需要图形界面或人工交互，微信侧暂不支持。")
	}
	if !flusher.SentAny() {
		diag.Info("bridge.runner.empty_text_fallback", "from_user_id", userID, "chat_id", userContext.ChatID)
		return s.sendText(ctx, wxClient, cred, userID, contextToken, "已处理，但未返回文本内容。")
	}
	diag.Info("bridge.runner.completed", "from_user_id", userID, "chat_id", userContext.ChatID, "result", "text_sent")
	return nil
}

func (s *Service) runTypingLoop(ctx context.Context, client *weixin.Client, cred state.Credential, userID, contextToken string) {
	cfg, err := client.GetConfig(ctx, cred.BotToken, userID, contextToken)
	if err != nil || cfg.TypingTicket == "" {
		if err != nil {
			diag.Debug("bridge.typing.config_failed", "from_user_id", userID, "error", err)
		}
		return
	}
	if err := client.SendTyping(ctx, cred.BotToken, userID, cfg.TypingTicket, true); err != nil {
		diag.Debug("bridge.typing.enable_failed", "from_user_id", userID, "error", err)
		return
	}
	diag.Debug("bridge.typing.enabled", "from_user_id", userID)
	ticker := time.NewTicker(typingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			diag.Debug("bridge.typing.loop_stopped", "from_user_id", userID)
			return
		case <-ticker.C:
			if err := client.SendTyping(ctx, cred.BotToken, userID, cfg.TypingTicket, true); err != nil {
				diag.Debug("bridge.typing.refresh_failed", "from_user_id", userID, "error", err)
				return
			}
		}
	}
}

func (s *Service) loadOrCreateUserContext(userID, contextToken string) (state.UserContext, error) {
	userContext, found, err := s.store.LoadUserContext(userID)
	if err != nil {
		return state.UserContext{}, err
	}
	createdChatID := false
	if !found || !isUUID(userContext.ChatID) {
		userContext.ChatID = newRunnerChatID()
		createdChatID = true
	}
	if strings.TrimSpace(contextToken) != "" {
		userContext.ContextToken = strings.TrimSpace(contextToken)
	}
	userContext.LastSeenAt = time.Now()
	if err := s.store.SaveUserContext(userID, userContext); err != nil {
		return state.UserContext{}, err
	}
	diag.Info("bridge.user_context.saved",
		"user_id", userID,
		"chat_id", userContext.ChatID,
		"context_token", diag.PreviewText(userContext.ContextToken, 32),
		"created_chat_id", createdChatID,
	)
	return userContext, nil
}

func (s *Service) stopTyping(ctx context.Context, client *weixin.Client, cred state.Credential, userID, contextToken string) error {
	cfg, err := client.GetConfig(ctx, cred.BotToken, userID, contextToken)
	if err != nil || cfg.TypingTicket == "" {
		return err
	}
	return client.SendTyping(ctx, cred.BotToken, userID, cfg.TypingTicket, false)
}

func (s *Service) sendText(ctx context.Context, client *weixin.Client, cred state.Credential, userID, contextToken, text string) error {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	startedAt := time.Now()
	diag.Info("bridge.weixin_send.start",
		"account_id", cred.AccountID,
		"to_user_id", userID,
		"context_token", diag.PreviewText(contextToken, 32),
		"text_len", utf8.RuneCountInString(text),
		"text_preview", diag.PreviewText(text, 160),
	)
	if err := client.SendText(ctx, cred.BotToken, userID, contextToken, text); err != nil {
		diag.Error("bridge.weixin_send.failed",
			"account_id", cred.AccountID,
			"to_user_id", userID,
			"context_token", diag.PreviewText(contextToken, 32),
			"duration_ms", diag.DurationMillis(startedAt),
			"error", err,
		)
		return err
	}
	diag.Info("bridge.weixin_send.success",
		"account_id", cred.AccountID,
		"to_user_id", userID,
		"context_token", diag.PreviewText(contextToken, 32),
		"duration_ms", diag.DurationMillis(startedAt),
	)
	s.saveStatus("bridge_weixin_send_success", func(status *state.AccountStatus) {
		status.Configured = true
		status.AccountID = cred.AccountID
		status.LastOutboundAt = time.Now()
		status.LastError = ""
	}, "to_user_id", userID)
	return nil
}

func (s *Service) recordStatusError(cred state.Credential, message string) {
	s.saveStatus("bridge_record_error", func(status *state.AccountStatus) {
		status.Configured = true
		status.Polling = true
		status.AccountID = cred.AccountID
		status.LastError = message
	}, "account_id", cred.AccountID, "last_error", diag.PreviewText(message, 160))
}

func (s *Service) resolveContextToken(userID, current string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	user, found, err := s.store.LoadUserContext(userID)
	if err != nil || !found {
		return ""
	}
	return user.ContextToken
}

func (s *Service) userLock(userID string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, ok := s.locks[userID]
	if ok {
		return lock
	}
	lock = &sync.Mutex{}
	s.locks[userID] = lock
	return lock
}

func extractText(msg weixin.Message) (string, bool) {
	for _, item := range msg.ItemList {
		if item.Type == 1 && item.TextItem != nil && strings.TrimSpace(item.TextItem.Text) != "" {
			return item.TextItem.Text, true
		}
	}
	return "", false
}

func isRichRunnerEvent(eventType string) bool {
	switch {
	case strings.HasPrefix(eventType, "tool."):
		return true
	case strings.HasPrefix(eventType, "action."):
		return true
	case eventType == "request.submit":
		return true
	case eventType == "request.steer":
		return true
	default:
		return false
	}
}

func newRunnerChatID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(buf[0])<<24|uint32(buf[1])<<16|uint32(buf[2])<<8|uint32(buf[3]),
		uint16(buf[4])<<8|uint16(buf[5]),
		uint16(buf[6])<<8|uint16(buf[7]),
		uint16(buf[8])<<8|uint16(buf[9]),
		uint64(buf[10])<<40|uint64(buf[11])<<32|uint64(buf[12])<<24|uint64(buf[13])<<16|uint64(buf[14])<<8|uint64(buf[15]),
	)
}

func isUUID(raw string) bool {
	if len(raw) != 36 {
		return false
	}
	for idx, ch := range raw {
		switch idx {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') && (ch < 'A' || ch > 'F') {
				return false
			}
		}
	}
	return true
}

type textFlusher struct {
	send    func(string) error
	buffer  strings.Builder
	timer   *time.Timer
	timerMu sync.Mutex
	mu      sync.Mutex
	sentAny bool
}

func newTextFlusher(send func(string) error) *textFlusher {
	return &textFlusher{send: send}
}

func (f *textFlusher) Add(delta string) error {
	if delta == "" {
		return nil
	}
	f.mu.Lock()
	f.buffer.WriteString(delta)
	shouldFlush := utf8.RuneCountInString(f.buffer.String()) >= flushRuneThreshold
	f.mu.Unlock()
	if shouldFlush {
		return f.Flush()
	}
	f.ensureTimer()
	return nil
}

func (f *textFlusher) Flush() error {
	f.timerMu.Lock()
	if f.timer != nil {
		f.timer.Stop()
		f.timer = nil
	}
	f.timerMu.Unlock()
	f.mu.Lock()
	text := strings.TrimSpace(f.buffer.String())
	f.buffer.Reset()
	f.mu.Unlock()
	if text == "" {
		return nil
	}
	if err := f.send(text); err != nil {
		return err
	}
	f.mu.Lock()
	f.sentAny = true
	f.mu.Unlock()
	return nil
}

func (f *textFlusher) SentAny() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sentAny
}

func (f *textFlusher) ensureTimer() {
	f.timerMu.Lock()
	defer f.timerMu.Unlock()
	if f.timer != nil {
		return
	}
	f.timer = time.AfterFunc(flushInterval, func() {
		if err := f.Flush(); err != nil {
			diag.Error("bridge.text_flush.failed", "error", err)
		}
	})
}

func (s *Service) saveStatus(reason string, mutate func(*state.AccountStatus), kv ...any) {
	status, err := s.store.LoadStatus()
	if err != nil {
		diag.Error("state.status.load_failed", "reason", reason, "error", err)
		return
	}
	before := status
	mutate(&status)
	if err := s.store.SaveStatus(status); err != nil {
		diag.Error("state.status.save_failed", "reason", reason, "error", err)
		return
	}
	fields := []any{
		"reason", reason,
		"polling_before", before.Polling,
		"polling_after", status.Polling,
		"last_inbound_after", status.LastInboundAt,
		"last_outbound_after", status.LastOutboundAt,
		"last_error_after", diag.PreviewText(status.LastError, 160),
		"account_id", status.AccountID,
	}
	fields = append(fields, kv...)
	diag.Info("state.status.updated", fields...)
}
