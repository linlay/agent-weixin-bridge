package runner

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"agent-weixin-bridge/internal/diag"
)

func TestParseSSEIgnoresDoneAndParsesRunnerEvents(t *testing.T) {
	stream := strings.NewReader("event: message\n" +
		"data: {\"type\":\"content.delta\",\"delta\":\"hello\"}\n\n" +
		"event: message\n" +
		"data: {\"type\":\"content.snapshot\",\"text\":\"hello world\"}\n\n" +
		"event: message\n" +
		"data: {\"type\":\"tool.start\",\"toolId\":\"tool-1\",\"runId\":\"run-1\"}\n\n" +
		"event: message\n" +
		"data: {\"type\":\"run.error\",\"runId\":\"run-1\",\"error\":{\"message\":\"runner exploded\"}}\n\n" +
		"event: message\n" +
		"data: {\"type\":\"run.complete\",\"runId\":\"run-1\"}\n\n" +
		"event: message\n" +
		"data: [DONE]\n\n")

	var events []StreamEvent
	err := parseSSE(stream, func(event StreamEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("parseSSE() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	if events[0].Type != "content.delta" || events[0].Delta != "hello" {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[1].Type != "content.snapshot" || events[1].Text != "hello world" {
		t.Fatalf("unexpected snapshot event: %+v", events[1])
	}
	if events[2].Type != "tool.start" || events[2].RunID != "run-1" {
		t.Fatalf("unexpected tool event: %+v", events[2])
	}
	if events[3].Type != "run.error" || events[3].ErrorMessage != "runner exploded" {
		t.Fatalf("unexpected error event: %+v", events[3])
	}
	if events[4].Type != "run.complete" {
		t.Fatalf("unexpected complete event: %+v", events[4])
	}
}

func TestStreamQueryLogsEventsWithoutLeakingBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(
			"data: {\"type\":\"content.delta\",\"delta\":\"hello from runner\"}\n\n" +
				"data: {\"type\":\"run.complete\",\"runId\":\"run-1\"}\n\n" +
				"data: [DONE]\n\n",
		))
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalWriter := log.Writer()
	originalFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(originalWriter)
		log.SetFlags(originalFlags)
	})
	diag.Configure("debug")

	client := NewClient(server.URL, "demo-agent", "super-secret-bearer")
	if err := client.StreamQuery(context.Background(), "chat-1", "hello runner", func(StreamEvent) error { return nil }); err != nil {
		t.Fatalf("StreamQuery() error = %v", err)
	}

	output := buf.String()
	for _, marker := range []string{
		"event=runner.query.start",
		"event=runner.query.response",
		"event=runner.query.event",
		"event=runner.query.complete",
	} {
		if !strings.Contains(output, marker) {
			t.Fatalf("missing log marker %q in %s", marker, output)
		}
	}
	if strings.Contains(output, "super-secret-bearer") {
		t.Fatalf("log output leaked bearer token: %s", output)
	}
}
