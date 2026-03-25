package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"agent-weixin-bridge/internal/diag"
)

type Client struct {
	baseURL     string
	agentKey    string
	bearerToken string
	http        *http.Client
}

type QueryRequest struct {
	ChatID   string `json:"chatId"`
	AgentKey string `json:"agentKey"`
	Role     string `json:"role"`
	Message  string `json:"message"`
	Stream   bool   `json:"stream,omitempty"`
}

type StreamEvent struct {
	Type         string
	Delta        string
	Text         string
	RunID        string
	ErrorMessage string
	Raw          json.RawMessage
}

func NewClient(baseURL, agentKey, bearerToken string) *Client {
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		agentKey:    strings.TrimSpace(agentKey),
		bearerToken: strings.TrimSpace(bearerToken),
		http: &http.Client{
			Timeout: 0,
		},
	}
}

func (c *Client) StreamQuery(ctx context.Context, chatID, message string, handle func(StreamEvent) error) error {
	startedAt := time.Now()
	diag.Info("runner.query.start",
		"base_url", c.baseURL,
		"path", "/api/query",
		"chat_id", chatID,
		"agent_key", c.agentKey,
		"message_len", utf8.RuneCountInString(message),
		"message_preview", diag.PreviewText(message, 160),
		"has_bearer", c.bearerToken != "",
	)
	reqBody, err := json.Marshal(QueryRequest{
		ChatID:   chatID,
		AgentKey: c.agentKey,
		Role:     "user",
		Message:  message,
		Stream:   true,
	})
	if err != nil {
		diag.Error("runner.query.marshal_failed", "chat_id", chatID, "error", err)
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/query", bytes.NewReader(reqBody))
	if err != nil {
		diag.Error("runner.query.request_build_failed", "chat_id", chatID, "error", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearerToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		diag.Error("runner.query.request_failed",
			"chat_id", chatID,
			"duration_ms", diag.DurationMillis(startedAt),
			"error", err,
		)
		return err
	}
	defer resp.Body.Close()
	diag.Info("runner.query.response",
		"chat_id", chatID,
		"status_code", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"),
		"duration_ms", diag.DurationMillis(startedAt),
	)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		err := fmt.Errorf("runner /api/query failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
		diag.Error("runner.query.bad_status",
			"chat_id", chatID,
			"status_code", resp.StatusCode,
			"body_preview", diag.PreviewText(strings.TrimSpace(string(body)), 200),
			"duration_ms", diag.DurationMillis(startedAt),
			"error", err,
		)
		return err
	}
	err = parseSSE(resp.Body, func(event StreamEvent) error {
		kv := []any{
			"chat_id", chatID,
			"type", event.Type,
			"run_id", event.RunID,
			"delta_len", utf8.RuneCountInString(event.Delta),
			"text_len", utf8.RuneCountInString(event.Text),
			"delta_preview", diag.PreviewText(event.Delta, 120),
			"text_preview", diag.PreviewText(event.Text, 120),
			"error_message", event.ErrorMessage,
		}
		if event.Type == "content.delta" {
			diag.Debug("runner.query.event", kv...)
		} else {
			diag.Info("runner.query.event", kv...)
		}
		return handle(event)
	})
	if err != nil {
		diag.Error("runner.query.stream_failed",
			"chat_id", chatID,
			"duration_ms", diag.DurationMillis(startedAt),
			"error", err,
		)
		return err
	}
	diag.Info("runner.query.complete", "chat_id", chatID, "duration_ms", diag.DurationMillis(startedAt))
	return nil
}

func parseSSE(r io.Reader, handle func(StreamEvent) error) error {
	reader := bufio.NewReaderSize(r, 128*1024)
	var dataLines []string
	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				dataLines = dataLines[:0]
				if payload != "[DONE]" {
					event, parseErr := parseStreamEvent([]byte(payload))
					if parseErr != nil {
						return parseErr
					}
					if err := handle(event); err != nil {
						return err
					}
				}
			}
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if err == io.EOF {
			if len(dataLines) > 0 {
				payload := strings.Join(dataLines, "\n")
				if payload != "[DONE]" {
					event, parseErr := parseStreamEvent([]byte(payload))
					if parseErr != nil {
						return parseErr
					}
					if err := handle(event); err != nil {
						return err
					}
				}
			}
			return nil
		}
	}
}

func parseStreamEvent(raw []byte) (StreamEvent, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		return StreamEvent{}, fmt.Errorf("parse runner SSE event: %w", err)
	}
	event := StreamEvent{
		Raw: append(json.RawMessage(nil), raw...),
	}

	_ = json.Unmarshal(payload["type"], &event.Type)
	_ = json.Unmarshal(payload["delta"], &event.Delta)
	_ = json.Unmarshal(payload["text"], &event.Text)
	_ = json.Unmarshal(payload["runId"], &event.RunID)
	if event.RunID == "" {
		_ = json.Unmarshal(payload["requestId"], &event.RunID)
	}
	event.ErrorMessage = parseErrorMessage(payload["error"])
	return event, nil
}

func parseErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var message string
	if err := json.Unmarshal(raw, &message); err == nil {
		return strings.TrimSpace(message)
	}

	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &payload); err == nil && strings.TrimSpace(payload.Message) != "" {
		return strings.TrimSpace(payload.Message)
	}

	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err == nil {
		if value, ok := generic["message"]; ok {
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return strings.TrimSpace(string(raw))
}
