package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"

	"agent-weixin-bridge/internal/diag"
	"agent-weixin-bridge/internal/state"
)

type stubCredentialStore struct {
	cred state.Credential
	ok   bool
	err  error
}

func (s stubCredentialStore) LoadCredential() (state.Credential, bool, error) {
	return s.cred, s.ok, s.err
}

type stubPollStarter struct {
	started bool
	err     error
}

func (s *stubPollStarter) Start(context.Context) error {
	s.started = true
	return s.err
}

func TestAttemptAutoStartPollStartsWhenCredentialExists(t *testing.T) {
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

	starter := &stubPollStarter{}
	attemptAutoStartPoll(context.Background(), true, stubCredentialStore{
		cred: state.Credential{
			BotToken:  "token-value",
			AccountID: "demo-account",
			BaseURL:   "https://ilinkai.weixin.qq.com",
		},
		ok: true,
	}, starter)

	if !starter.started {
		t.Fatalf("expected Start to be called")
	}
	output := buf.String()
	if !strings.Contains(output, "event=poll.auto_start.attempt") {
		t.Fatalf("missing attempt log: %s", output)
	}
	if !strings.Contains(output, "event=poll.auto_start.started") {
		t.Fatalf("missing started log: %s", output)
	}
}

func TestAttemptAutoStartPollLogsFailure(t *testing.T) {
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

	starter := &stubPollStarter{err: errors.New("runner down")}
	attemptAutoStartPoll(context.Background(), true, stubCredentialStore{
		cred: state.Credential{
			BotToken:  "token-value",
			AccountID: "demo-account",
			BaseURL:   "https://ilinkai.weixin.qq.com",
		},
		ok: true,
	}, starter)

	if !strings.Contains(buf.String(), "event=poll.auto_start.failed") {
		t.Fatalf("missing failed log: %s", buf.String())
	}
}
