package main

import (
	"context"
	"strings"

	"agent-weixin-bridge/internal/diag"
	"agent-weixin-bridge/internal/state"
)

type credentialStore interface {
	LoadCredential() (state.Credential, bool, error)
}

type pollStarter interface {
	Start(context.Context) error
}

func attemptAutoStartPoll(ctx context.Context, enabled bool, store credentialStore, starter pollStarter) {
	if !enabled {
		diag.Info("poll.auto_start.disabled")
		return
	}

	cred, ok, err := store.LoadCredential()
	if err != nil {
		diag.Error("poll.auto_start.load_credential_failed", "error", err)
		return
	}
	if !ok || strings.TrimSpace(cred.BotToken) == "" {
		diag.Info("poll.auto_start.skipped", "reason", "missing_credential")
		return
	}

	diag.Info("poll.auto_start.attempt", "account_id", cred.AccountID, "base_url", cred.BaseURL)
	if err := starter.Start(ctx); err != nil {
		diag.Error("poll.auto_start.failed", "account_id", cred.AccountID, "error", err)
		return
	}
	diag.Info("poll.auto_start.started", "account_id", cred.AccountID)
}
