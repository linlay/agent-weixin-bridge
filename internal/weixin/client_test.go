package weixin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var samplePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x04, 0x00, 0x00, 0x00, 0xb5, 0x1c, 0x0c,
	0x02, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41,
	0x54, 0x78, 0xda, 0x63, 0xfc, 0xff, 0x1f, 0x00,
	0x03, 0x03, 0x02, 0x00, 0xef, 0xef, 0x8d, 0x13,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}

func TestResolveQRImageFromDataURL(t *testing.T) {
	client := NewClient("http://127.0.0.1")
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(samplePNG)

	image, err := client.ResolveQRImage(context.Background(), dataURL)
	if err != nil {
		t.Fatalf("ResolveQRImage() error = %v", err)
	}
	if image.ContentType != "image/png" {
		t.Fatalf("ContentType = %q, want %q", image.ContentType, "image/png")
	}
	if len(image.Data) == 0 {
		t.Fatalf("image data is empty")
	}
}

func TestResolveQRImageFromBareBase64(t *testing.T) {
	client := NewClient("http://127.0.0.1")
	raw := base64.StdEncoding.EncodeToString(samplePNG)

	image, err := client.ResolveQRImage(context.Background(), raw)
	if err != nil {
		t.Fatalf("ResolveQRImage() error = %v", err)
	}
	if image.ContentType != "image/png" {
		t.Fatalf("ContentType = %q, want %q", image.ContentType, "image/png")
	}
	if len(image.Data) == 0 {
		t.Fatalf("image data is empty")
	}
}

func TestResolveQRImageFromHTTPURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(samplePNG)
	}))
	defer server.Close()

	client := NewClient("http://127.0.0.1")
	image, err := client.ResolveQRImage(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("ResolveQRImage() error = %v", err)
	}
	if image.ContentType != "image/png" {
		t.Fatalf("ContentType = %q, want %q", image.ContentType, "image/png")
	}
	if len(image.Data) != len(samplePNG) {
		t.Fatalf("len(image.Data) = %d, want %d", len(image.Data), len(samplePNG))
	}
}

func TestResolveQRImageRejectsUnknownFormat(t *testing.T) {
	client := NewClient("http://127.0.0.1")

	_, err := client.ResolveQRImage(context.Background(), "not-a-qrcode")
	if err == nil {
		t.Fatalf("ResolveQRImage() error = nil, want error")
	}
}

func TestSendTextAcceptsSuccessfulBusinessAck(t *testing.T) {
	type sendTextPayload struct {
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

	var gotPayload sendTextPayload
	var gotAuthorizationType string
	var gotAuthorization string
	var gotWechatUIN string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ilink/bot/sendmessage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		gotAuthorizationType = r.Header.Get("AuthorizationType")
		gotAuthorization = r.Header.Get("Authorization")
		gotWechatUIN = r.Header.Get("X-WECHAT-UIN")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode sendmessage payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AckResponse{Ret: 0, ErrCode: 0})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.SendText(context.Background(), "bot-token", "user-1", "ctx-1", "hello"); err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	if gotAuthorizationType != "ilink_bot_token" {
		t.Fatalf("AuthorizationType = %q, want %q", gotAuthorizationType, "ilink_bot_token")
	}
	if gotAuthorization != "Bearer bot-token" {
		t.Fatalf("Authorization = %q, want %q", gotAuthorization, "Bearer bot-token")
	}
	if strings.TrimSpace(gotWechatUIN) == "" {
		t.Fatalf("X-WECHAT-UIN is empty")
	}
	if gotPayload.Msg.FromUserID != "" {
		t.Fatalf("from_user_id = %q, want empty", gotPayload.Msg.FromUserID)
	}
	if gotPayload.Msg.ToUserID != "user-1" {
		t.Fatalf("to_user_id = %q, want %q", gotPayload.Msg.ToUserID, "user-1")
	}
	if !strings.HasPrefix(gotPayload.Msg.ClientID, "chat-") {
		t.Fatalf("client_id = %q, want chat-*", gotPayload.Msg.ClientID)
	}
	if gotPayload.Msg.MessageType != outboundMessageType {
		t.Fatalf("message_type = %d, want %d", gotPayload.Msg.MessageType, outboundMessageType)
	}
	if gotPayload.Msg.MessageState != outboundMessageState {
		t.Fatalf("message_state = %d, want %d", gotPayload.Msg.MessageState, outboundMessageState)
	}
	if gotPayload.Msg.ContextToken != "ctx-1" {
		t.Fatalf("context_token = %q, want %q", gotPayload.Msg.ContextToken, "ctx-1")
	}
	if len(gotPayload.Msg.ItemList) != 1 {
		t.Fatalf("item_list length = %d, want 1", len(gotPayload.Msg.ItemList))
	}
	if gotPayload.Msg.ItemList[0].Type != messageItemText {
		t.Fatalf("item_list[0].type = %d, want %d", gotPayload.Msg.ItemList[0].Type, messageItemText)
	}
	if gotPayload.Msg.ItemList[0].TextItem.Text != "hello" {
		t.Fatalf("text_item.text = %q, want %q", gotPayload.Msg.ItemList[0].TextItem.Text, "hello")
	}
	if gotPayload.BaseInfo.ChannelVersion != channelVersion {
		t.Fatalf("base_info.channel_version = %q, want %q", gotPayload.BaseInfo.ChannelVersion, channelVersion)
	}
}

func TestSendTextRejectsBusinessErrorAck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AckResponse{Ret: 1, ErrCode: 40013, ErrMessage: "invalid msg format"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.SendText(context.Background(), "bot-token", "user-1", "ctx-1", "hello")
	if err == nil {
		t.Fatalf("SendText() error = nil, want business error")
	}
	if !strings.Contains(err.Error(), "invalid msg format") {
		t.Fatalf("SendText() error = %v, want errmsg included", err)
	}
}

func TestSendTypingRejectsBusinessErrorAck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AckResponse{Ret: 1, ErrCode: 30002, ErrMessage: "typing ticket expired"})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.SendTyping(context.Background(), "bot-token", "user-1", "ticket-1", true)
	if err == nil {
		t.Fatalf("SendTyping() error = nil, want business error")
	}
	if !strings.Contains(err.Error(), "typing ticket expired") {
		t.Fatalf("SendTyping() error = %v, want errmsg included", err)
	}
}

func TestSendTextGeneratesUniqueClientIDs(t *testing.T) {
	var clientIDs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload struct {
			Msg struct {
				ClientID string `json:"client_id"`
			} `json:"msg"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode sendmessage payload: %v", err)
		}
		clientIDs = append(clientIDs, payload.Msg.ClientID)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(AckResponse{Ret: 0, ErrCode: 0})
	}))
	defer server.Close()

	client := NewClient(server.URL)
	if err := client.SendText(context.Background(), "bot-token", "user-1", "ctx-1", "hello"); err != nil {
		t.Fatalf("first SendText() error = %v", err)
	}
	if err := client.SendText(context.Background(), "bot-token", "user-1", "ctx-1", "hello again"); err != nil {
		t.Fatalf("second SendText() error = %v", err)
	}
	if len(clientIDs) != 2 {
		t.Fatalf("captured %d client IDs, want 2", len(clientIDs))
	}
	if clientIDs[0] == "" || clientIDs[1] == "" {
		t.Fatalf("client IDs must be non-empty: %#v", clientIDs)
	}
	if clientIDs[0] == clientIDs[1] {
		t.Fatalf("client IDs must be unique: %#v", clientIDs)
	}
}
