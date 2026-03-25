package weixin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"agent-weixin-bridge/internal/diag"
)

const (
	messageItemText      = 1
	outboundMessageType  = 2
	outboundMessageState = 2
	typingStatusOn       = 1
	typingStatusOff      = 2
	maxImageBytes        = 8 << 20
	channelVersion       = "1.0.2"
)

type Client struct {
	baseURL string
	http    *http.Client
}

type QRImage struct {
	ContentType string
	Data        []byte
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{},
	}
}

func (c *Client) ForBaseURL(baseURL string) *Client {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(baseURL) == c.baseURL {
		return c
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    c.http,
	}
}

func (c *Client) StartQRLogin(ctx context.Context, botType string) (QRStartResponse, error) {
	var resp QRStartResponse
	path := fmt.Sprintf("/ilink/bot/get_bot_qrcode?bot_type=%s", url.QueryEscape(botType))
	if err := c.getJSON(ctx, path, nil, &resp); err != nil {
		return QRStartResponse{}, err
	}
	return resp, nil
}

func (c *Client) GetQRLoginStatus(ctx context.Context, qrcode string) (QRStatusResponse, error) {
	var resp QRStatusResponse
	path := fmt.Sprintf("/ilink/bot/get_qrcode_status?qrcode=%s", url.QueryEscape(qrcode))
	headers := map[string]string{"iLink-App-ClientVersion": "1"}
	if err := c.getJSON(ctx, path, headers, &resp); err != nil {
		return QRStatusResponse{}, err
	}
	return resp, nil
}

func (c *Client) GetUpdates(ctx context.Context, token, cursor string) (GetUpdatesResponse, error) {
	var resp GetUpdatesResponse
	if err := c.postJSON(ctx, "/ilink/bot/getupdates", token, withBaseInfo(GetUpdatesRequest{GetUpdatesBuf: cursor}), &resp); err != nil {
		return GetUpdatesResponse{}, err
	}
	return resp, nil
}

func (c *Client) SendText(ctx context.Context, token, toUserID, contextToken, text string) error {
	clientID := generateClientID()
	req := SendMessageRequest{
		Msg: OutboundMessage{
			FromUserID:   "",
			ToUserID:     toUserID,
			ClientID:     clientID,
			MessageType:  outboundMessageType,
			MessageState: outboundMessageState,
			ContextToken: contextToken,
			ItemList: []OutboundItem{{
				Type:     messageItemText,
				TextItem: &OutboundText{Text: text},
			}},
		},
	}
	diag.Info("weixin.sendmessage.request",
		"to_user_id", toUserID,
		"client_id", clientID,
		"message_type", req.Msg.MessageType,
		"message_state", req.Msg.MessageState,
		"channel_version", channelVersion,
		"has_context_token", strings.TrimSpace(contextToken) != "",
		"context_token", diag.PreviewText(contextToken, 32),
		"item_count", len(req.Msg.ItemList),
		"text_len", utf8.RuneCountInString(text),
		"text_preview", diag.PreviewText(text, 160),
	)
	return c.postAck(ctx, "/ilink/bot/sendmessage", token, withBaseInfo(req), map[string]any{
		"to_user_id":        toUserID,
		"client_id":         clientID,
		"message_type":      req.Msg.MessageType,
		"message_state":     req.Msg.MessageState,
		"channel_version":   channelVersion,
		"has_context_token": strings.TrimSpace(contextToken) != "",
		"context_token":     diag.PreviewText(contextToken, 32),
	})
}

func (c *Client) GetConfig(ctx context.Context, token, userID, contextToken string) (GetConfigResponse, error) {
	var resp GetConfigResponse
	req := GetConfigRequest{
		ILinkUserID:  userID,
		ContextToken: contextToken,
	}
	if err := c.postJSON(ctx, "/ilink/bot/getconfig", token, withBaseInfo(req), &resp); err != nil {
		return GetConfigResponse{}, err
	}
	return resp, nil
}

func (c *Client) SendTyping(ctx context.Context, token, userID, ticket string, enabled bool) error {
	status := typingStatusOff
	if enabled {
		status = typingStatusOn
	}
	req := SendTypingRequest{
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
	}
	return c.postAck(ctx, "/ilink/bot/sendtyping", token, withBaseInfo(req), map[string]any{
		"user_id": userID,
		"status":  status,
	})
}

func (c *Client) ResolveQRImage(ctx context.Context, raw string) (QRImage, error) {
	value := NormalizeQRCodeValue(raw)
	if value == "" {
		return QRImage{}, fmt.Errorf("qrcode image is empty")
	}

	switch ClassifyQRCodeValue(value) {
	case QRCodeValueDataURL:
		return decodeDataURL(value)
	case QRCodeValueBase64Image:
		data, err := base64.StdEncoding.DecodeString(compactBase64(value))
		if err != nil {
			return QRImage{}, fmt.Errorf("decode qrcode base64: %w", err)
		}
		return QRImage{
			ContentType: "image/png",
			Data:        data,
		}, nil
	case QRCodeValueHTTPURL, QRCodeValueLiteAppURL:
		return c.fetchQRImage(ctx, value)
	default:
		return QRImage{}, fmt.Errorf("unsupported qrcode image format")
	}
}

func (c *Client) fetchQRImage(ctx context.Context, imageURL string) (QRImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return QRImage{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return QRImage{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return QRImage{}, fmt.Errorf("fetch qrcode image failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageBytes+1))
	if err != nil {
		return QRImage{}, err
	}
	if len(data) > maxImageBytes {
		return QRImage{}, fmt.Errorf("qrcode image too large")
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	normalizedContentType := NormalizeImageContentType(contentType)
	if !strings.HasPrefix(strings.ToLower(normalizedContentType), "image/") {
		return QRImage{}, fmt.Errorf("qrcode URL returned non-image content-type %q", normalizedContentType)
	}
	return QRImage{
		ContentType: normalizedContentType,
		Data:        data,
	}, nil
}

func (c *Client) getJSON(ctx context.Context, path string, extraHeaders map[string]string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("weixin GET %s failed: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if target == nil {
		io.Copy(io.Discard, resp.Body)
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) postJSON(ctx context.Context, path, token string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("weixin POST %s failed: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if target == nil {
		return c.decodeAck(path, resp.Body, nil)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *Client) postAck(ctx context.Context, path, token string, payload any, metadata map[string]any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	startedAt := time.Now()
	resp, err := c.http.Do(req)
	if err != nil {
		diag.Error(strings.TrimPrefix(strings.ReplaceAll(path, "/", "."), ".")+".request_failed",
			"duration_ms", diag.DurationMillis(startedAt),
			"error", err,
		)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		err := fmt.Errorf("weixin POST %s failed: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(raw)))
		fields := []any{"status_code", resp.StatusCode, "duration_ms", diag.DurationMillis(startedAt), "error", err}
		fields = append(fields, metadataToKV(metadata)...)
		diag.Error(strings.TrimPrefix(strings.ReplaceAll(path, "/", "."), ".")+".http_error", fields...)
		return err
	}

	fields := []any{
		"status_code", resp.StatusCode,
		"duration_ms", diag.DurationMillis(startedAt),
	}
	fields = append(fields, metadataToKV(metadata)...)
	return c.decodeAck(path, resp.Body, fields)
}

func (c *Client) decodeAck(path string, body io.Reader, metadata []any) error {
	var ack AckResponse
	if err := json.NewDecoder(body).Decode(&ack); err != nil {
		fields := append([]any{}, metadata...)
		fields = append(fields, "error", err)
		diag.Error(strings.TrimPrefix(strings.ReplaceAll(path, "/", "."), ".")+".decode_error", fields...)
		return fmt.Errorf("decode weixin %s ack: %w", path, err)
	}

	fields := append([]any{}, metadata...)
	fields = append(fields,
		"ret", ack.Ret,
		"errcode", ack.ErrCode,
		"errmsg", ack.ErrMessage,
	)
	diag.Info(strings.TrimPrefix(strings.ReplaceAll(path, "/", "."), ".")+".response", fields...)

	if ack.Ret != 0 || ack.ErrCode != 0 {
		err := fmt.Errorf("weixin %s business error: ret=%d errcode=%d errmsg=%s", path, ack.Ret, ack.ErrCode, strings.TrimSpace(ack.ErrMessage))
		fields = append(fields, "error", err)
		diag.Error(strings.TrimPrefix(strings.ReplaceAll(path, "/", "."), ".")+".business_error", fields...)
		return err
	}
	return nil
}

func metadataToKV(metadata map[string]any) []any {
	if len(metadata) == 0 {
		return nil
	}
	fields := make([]any, 0, len(metadata)*2)
	for key, value := range metadata {
		fields = append(fields, key, value)
	}
	return fields
}

func randomWechatUIN() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		binary.BigEndian.PutUint32(buf[:], uint32(time.Now().UnixNano()))
	}
	value := binary.BigEndian.Uint32(buf[:])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", value)))
}

func generateClientID() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		binary.BigEndian.PutUint32(buf[:], uint32(time.Now().UnixNano()))
	}
	return fmt.Sprintf("chat-%d-%x", time.Now().UnixMilli(), buf[:])
}

func withBaseInfo(payload any) map[string]any {
	merged := map[string]any{
		"base_info": map[string]any{
			"channel_version": channelVersion,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return merged
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return merged
	}
	for key, value := range fields {
		merged[key] = value
	}
	return merged
}

func decodeDataURL(value string) (QRImage, error) {
	comma := strings.IndexByte(value, ',')
	if comma < 0 {
		return QRImage{}, fmt.Errorf("invalid data url")
	}
	meta := value[:comma]
	payload := value[comma+1:]
	if !strings.Contains(meta, ";base64") {
		return QRImage{}, fmt.Errorf("data url is not base64 encoded")
	}
	contentType := strings.TrimPrefix(strings.Split(meta, ";")[0], "data:")
	if strings.TrimSpace(contentType) == "" {
		contentType = "image/png"
	}
	data, err := base64.StdEncoding.DecodeString(compactBase64(payload))
	if err != nil {
		return QRImage{}, fmt.Errorf("decode data url: %w", err)
	}
	return QRImage{
		ContentType: contentType,
		Data:        data,
	}, nil
}

func looksLikeBase64Image(value string) bool {
	if value == "" {
		return false
	}
	normalized := compactBase64(value)
	if len(normalized) < 80 || len(normalized)%4 != 0 {
		return false
	}
	for _, ch := range normalized {
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '+' || ch == '/' || ch == '=' {
			continue
		}
		return false
	}
	return true
}

func compactBase64(value string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', ' ':
			return -1
		default:
			return r
		}
	}, value)
}

func NormalizeImageContentType(value string) string {
	contentType := strings.TrimSpace(value)
	if contentType == "" {
		return "image/png"
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || strings.TrimSpace(mediaType) == "" {
		return contentType
	}
	return mediaType
}
