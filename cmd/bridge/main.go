package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"agent-weixin-bridge/internal/bridge"
	"agent-weixin-bridge/internal/config"
	"agent-weixin-bridge/internal/diag"
	"agent-weixin-bridge/internal/httpapi"
	"agent-weixin-bridge/internal/runner"
	"agent-weixin-bridge/internal/state"
	"agent-weixin-bridge/internal/weixin"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	diag.Configure(cfg.LogLevel)
	diag.Info("bridge.config.loaded",
		"bridge_http_addr", cfg.BridgeHTTPAddr,
		"weixin_base_url", cfg.WeixinBaseURL,
		"weixin_bot_type", cfg.WeixinBotType,
		"runner_base_url", cfg.RunnerBaseURL,
		"runner_agent_key", cfg.RunnerAgentKey,
		"runner_has_bearer", cfg.RunnerBearerToken != "",
		"state_dir", cfg.StateDir,
		"log_level", cfg.LogLevel,
		"auto_start_poll", cfg.AutoStartPoll,
	)

	store, err := state.NewFileStore(cfg.StateDir)
	if err != nil {
		log.Fatalf("init store: %v", err)
	}

	wxClient := weixin.NewClient(cfg.WeixinBaseURL)
	loginService := weixin.NewLoginService(wxClient, store, cfg.WeixinBotType)
	runnerClient := runner.NewClient(cfg.RunnerBaseURL, cfg.RunnerAgentKey, cfg.RunnerBearerToken)
	bridgeService := bridge.NewService(wxClient, runnerClient, store)
	pollManager := weixin.NewPollManager(wxClient, store, bridgeService, cfg.WeixinBotType)

	attemptAutoStartPoll(context.Background(), cfg.AutoStartPoll, store, pollManager)

	handler := httpapi.NewServer(httpapi.Dependencies{
		Config:       cfg,
		Store:        store,
		LoginService: loginService,
		PollManager:  pollManager,
	})

	server := &http.Server{
		Addr:    cfg.BridgeHTTPAddr,
		Handler: handler.Routes(),
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		diag.Info("bridge.http.listen", "addr", cfg.BridgeHTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http listen: %v", err)
		}
	}()

	<-shutdownCtx.Done()
	diag.Info("bridge.shutdown.start")
	pollManager.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
	diag.Info("bridge.shutdown.complete")
}
