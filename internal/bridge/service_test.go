package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"agent-weixin-bridge/internal/runner"
	"agent-weixin-bridge/internal/state"
	"agent-weixin-bridge/internal/weixin"
)

func TestTextFlusherFlushesOnThreshold(t *testing.T) {
	var chunks []string
	flusher := newTextFlusher(func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})

	if err := flusher.Add(strings.Repeat("a", flushRuneThreshold)); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != strings.Repeat("a", flushRuneThreshold) {
		t.Fatalf("unexpected chunk %q", chunks[0])
	}
}

func TestTextFlusherFlushesOnTimer(t *testing.T) {
	var chunks []string
	flusher := newTextFlusher(func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})

	if err := flusher.Add("hello"); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	time.Sleep(flushInterval + 300*time.Millisecond)
	if len(chunks) != 1 {
		t.Fatalf("expected timer flush to emit 1 chunk, got %d", len(chunks))
	}
	if chunks[0] != "hello" {
		t.Fatalf("unexpected chunk %q", chunks[0])
	}
}

func TestHandleInboundAssignsUUIDChatIDAndStreamsText(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"content.delta\",\"delta\":\"hello from runner\"}\n\n",
		"data: {\"type\":\"run.complete\",\"runId\":\"run-1\"}\n\n",
		"data: [DONE]\n\n",
	})
	defer harness.Close()

	if err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-1", "ctx-1", "你好")); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	userContext, found, err := harness.store.LoadUserContext("user-1")
	if err != nil {
		t.Fatalf("LoadUserContext() error = %v", err)
	}
	if !found {
		t.Fatalf("expected user context to be saved")
	}
	if !isUUID(userContext.ChatID) {
		t.Fatalf("expected UUID chat ID, got %q", userContext.ChatID)
	}
	if harness.runnerRequest.ChatID != userContext.ChatID {
		t.Fatalf("runner chat ID = %q, want %q", harness.runnerRequest.ChatID, userContext.ChatID)
	}
	if harness.runnerRequest.AgentKey != "demo-agent" {
		t.Fatalf("runner agent key = %q", harness.runnerRequest.AgentKey)
	}
	if got := harness.sentTexts(); len(got) != 1 || got[0] != "hello from runner" {
		t.Fatalf("unexpected outbound texts: %#v", got)
	}
	messages := harness.sentMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 outbound message, got %d", len(messages))
	}
	if messages[0].FromUserID != "" {
		t.Fatalf("from_user_id = %q, want empty", messages[0].FromUserID)
	}
	if messages[0].ToUserID != "user-1" {
		t.Fatalf("to_user_id = %q, want %q", messages[0].ToUserID, "user-1")
	}
	if !strings.HasPrefix(messages[0].ClientID, "chat-") {
		t.Fatalf("client_id = %q, want chat-*", messages[0].ClientID)
	}
	if messages[0].MessageType != 2 {
		t.Fatalf("message_type = %d, want 2", messages[0].MessageType)
	}
	if messages[0].MessageState != 2 {
		t.Fatalf("message_state = %d, want 2", messages[0].MessageState)
	}
	if messages[0].ChannelVersion != "1.0.2" {
		t.Fatalf("channel_version = %q, want %q", messages[0].ChannelVersion, "1.0.2")
	}
}

func TestHandleInboundMigratesLegacyChatIDAndPreservesContextToken(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"content.delta\",\"delta\":\"迁移成功\"}\n\n",
		"data: {\"type\":\"run.complete\",\"runId\":\"run-2\"}\n\n",
		"data: [DONE]\n\n",
	})
	defer harness.Close()

	if err := harness.store.SaveUserContext("user-legacy", state.UserContext{
		ChatID:       "weixin:user-legacy",
		ContextToken: "legacy-context",
		LastSeenAt:   time.Now().Add(-time.Minute),
	}); err != nil {
		t.Fatalf("SaveUserContext() error = %v", err)
	}

	if err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-legacy", "", "继续聊")); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	userContext, found, err := harness.store.LoadUserContext("user-legacy")
	if err != nil {
		t.Fatalf("LoadUserContext() error = %v", err)
	}
	if !found {
		t.Fatalf("expected user context to be saved")
	}
	if !isUUID(userContext.ChatID) {
		t.Fatalf("expected migrated UUID chat ID, got %q", userContext.ChatID)
	}
	if userContext.ChatID == "weixin:user-legacy" {
		t.Fatalf("legacy chat ID was not migrated")
	}
	if userContext.ContextToken != "legacy-context" {
		t.Fatalf("context token = %q, want %q", userContext.ContextToken, "legacy-context")
	}
	if harness.runnerRequest.ChatID != userContext.ChatID {
		t.Fatalf("runner chat ID = %q, want %q", harness.runnerRequest.ChatID, userContext.ChatID)
	}
	if got := harness.sentContextTokens(); len(got) != 1 || got[0] != "legacy-context" {
		t.Fatalf("unexpected outbound context tokens: %#v", got)
	}
	if messages := harness.sentMessages(); len(messages) != 1 || messages[0].ClientID == "" {
		t.Fatalf("unexpected outbound message metadata: %#v", messages)
	}
}

func TestHandleInboundUsesSnapshotFallback(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"content.snapshot\",\"text\":\"完整答案\"}\n\n",
		"data: {\"type\":\"run.complete\",\"runId\":\"run-3\"}\n\n",
		"data: [DONE]\n\n",
	})
	defer harness.Close()

	if err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-snapshot", "ctx-snapshot", "请总结")); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	if got := harness.sentTexts(); len(got) != 1 || got[0] != "完整答案" {
		t.Fatalf("unexpected outbound texts: %#v", got)
	}
}

func TestHandleInboundDegradesRichEventsWithoutText(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"tool.start\",\"toolId\":\"tool-1\",\"runId\":\"run-4\"}\n\n",
		"data: {\"type\":\"request.submit\",\"toolId\":\"tool-1\",\"runId\":\"run-4\",\"chatId\":\"123e4567-e89b-12d3-a456-426614174000\"}\n\n",
		"data: {\"type\":\"run.complete\",\"runId\":\"run-4\"}\n\n",
		"data: [DONE]\n\n",
	})
	defer harness.Close()

	if err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-rich", "ctx-rich", "帮我点一下")); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	want := "当前该智能体需要图形界面或人工交互，微信侧暂不支持。"
	if got := harness.sentTexts(); len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected outbound texts: %#v", got)
	}
}

func TestHandleInboundReportsRunErrorAndStoresLastError(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"run.error\",\"runId\":\"run-5\",\"error\":{\"message\":\"需要人工确认\"}}\n\n",
		"data: [DONE]\n\n",
	})
	defer harness.Close()

	if err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-error", "ctx-error", "执行一下")); err != nil {
		t.Fatalf("HandleInbound() error = %v", err)
	}

	want := "微信桥接到 runner 失败：需要人工确认"
	if got := harness.sentTexts(); len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected outbound texts: %#v", got)
	}
	status, err := harness.store.LoadStatus()
	if err != nil {
		t.Fatalf("LoadStatus() error = %v", err)
	}
	if status.LastError != want {
		t.Fatalf("status.LastError = %q, want %q", status.LastError, want)
	}
}

func TestHandleInboundPropagatesWeixinSendBusinessError(t *testing.T) {
	harness := newInboundHarness(t, []string{
		"data: {\"type\":\"content.delta\",\"delta\":\"runner answered\"}\n\n",
		"data: {\"type\":\"run.complete\",\"runId\":\"run-6\"}\n\n",
		"data: [DONE]\n\n",
	})
	harness.sendMessageAck = `{"ret":1,"errcode":40013,"errmsg":"invalid msg format"}`
	defer harness.Close()

	err := harness.service.HandleInbound(context.Background(), harness.cred, inboundTextMessage("user-send-error", "ctx-send-error", "执行一下"))
	if err == nil {
		t.Fatalf("HandleInbound() error = nil, want sendmessage business error")
	}
	if !strings.Contains(err.Error(), "invalid msg format") {
		t.Fatalf("HandleInbound() error = %v, want business errmsg", err)
	}

	status, statusErr := harness.store.LoadStatus()
	if statusErr != nil {
		t.Fatalf("LoadStatus() error = %v", statusErr)
	}
	if !strings.Contains(status.LastError, "invalid msg format") {
		t.Fatalf("status.LastError = %q, want invalid msg format", status.LastError)
	}
	if !status.LastOutboundAt.IsZero() {
		t.Fatalf("LastOutboundAt = %v, want zero when sendmessage business error", status.LastOutboundAt)
	}
}

type inboundHarness struct {
	service       *Service
	store         *state.FileStore
	cred          state.Credential
	runnerRequest runner.QueryRequest

	wxServer       *httptest.Server
	runnerServer   *httptest.Server
	sendMessageAck string

	mu       sync.Mutex
	messages []sentMessage
}

type sentMessage struct {
	FromUserID     string
	ToUserID       string
	ClientID       string
	MessageType    int
	MessageState   int
	ContextToken   string
	Text           string
	ChannelVersion string
}

func newInboundHarness(t *testing.T, runnerEvents []string) *inboundHarness {
	t.Helper()

	h := &inboundHarness{
		sendMessageAck: `{"ret":0,"errcode":0}`,
	}
	h.wxServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/getconfig":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ret":0,"typing_ticket":""}`))
		case "/ilink/bot/sendmessage":
			defer r.Body.Close()
			var payload struct {
				Msg struct {
					FromUserID   string `json:"from_user_id"`
					ToUserID     string `json:"to_user_id"`
					ClientID     string `json:"client_id"`
					MessageType  int    `json:"message_type"`
					MessageState int    `json:"message_state"`
					ContextToken string `json:"context_token"`
					ItemList     []struct {
						Type     int `json:"type"`
						TextItem struct {
							Text string `json:"text"`
						} `json:"text_item"`
					} `json:"item_list"`
				} `json:"msg"`
				BaseInfo struct {
					ChannelVersion string `json:"channel_version"`
				} `json:"base_info"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode weixin sendmessage payload: %v", err)
			}
			text := ""
			if len(payload.Msg.ItemList) > 0 {
				text = payload.Msg.ItemList[0].TextItem.Text
			}
			h.mu.Lock()
			h.messages = append(h.messages, sentMessage{
				FromUserID:     payload.Msg.FromUserID,
				ToUserID:       payload.Msg.ToUserID,
				ClientID:       payload.Msg.ClientID,
				MessageType:    payload.Msg.MessageType,
				MessageState:   payload.Msg.MessageState,
				ContextToken:   payload.Msg.ContextToken,
				Text:           text,
				ChannelVersion: payload.BaseInfo.ChannelVersion,
			})
			h.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(h.sendMessageAck))
		default:
			t.Fatalf("unexpected weixin path: %s", r.URL.Path)
		}
	}))

	h.runnerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/query" {
			t.Fatalf("unexpected runner path: %s", r.URL.Path)
		}
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&h.runnerRequest); err != nil {
			t.Fatalf("decode runner request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, event := range runnerEvents {
			_, _ = w.Write([]byte(event))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))

	store, err := state.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	h.store = store
	h.service = NewService(
		weixin.NewClient(h.wxServer.URL),
		runner.NewClient(h.runnerServer.URL, "demo-agent", ""),
		store,
	)
	h.cred = state.Credential{
		BotToken:  "bot-token",
		BaseURL:   h.wxServer.URL,
		AccountID: "wx-account",
	}
	return h
}

func (h *inboundHarness) sentTexts() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	texts := make([]string, 0, len(h.messages))
	for _, message := range h.messages {
		texts = append(texts, message.Text)
	}
	return texts
}

func (h *inboundHarness) sentContextTokens() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	tokens := make([]string, 0, len(h.messages))
	for _, message := range h.messages {
		tokens = append(tokens, message.ContextToken)
	}
	return tokens
}

func (h *inboundHarness) sentMessages() []sentMessage {
	h.mu.Lock()
	defer h.mu.Unlock()
	messages := make([]sentMessage, len(h.messages))
	copy(messages, h.messages)
	return messages
}

func (h *inboundHarness) Close() {
	if h.runnerServer != nil {
		h.runnerServer.Close()
	}
	if h.wxServer != nil {
		h.wxServer.Close()
	}
}

func inboundTextMessage(userID, contextToken, text string) weixin.Message {
	return weixin.Message{
		FromUserID:   userID,
		ContextToken: contextToken,
		MessageType:  1,
		ItemList: []weixin.MessageItem{{
			Type: 1,
			TextItem: &weixin.TextItem{
				Text: text,
			},
		}},
	}
}
