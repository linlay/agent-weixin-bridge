package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"agent-weixin-bridge/internal/config"
	"agent-weixin-bridge/internal/state"
	"agent-weixin-bridge/internal/weixin"
)

type Dependencies struct {
	Config       config.Config
	Store        *state.FileStore
	LoginService *weixin.LoginService
	PollManager  *weixin.PollManager
}

type Server struct {
	deps Dependencies
	mux  *http.ServeMux
}

func NewServer(deps Dependencies) *Server {
	s := &Server{
		deps: deps,
		mux:  http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

func (s *Server) Routes() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /", s.home)
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /assets/qrcode.js", s.qrCodeJS)
	s.mux.HandleFunc("POST /api/weixin/login/start", s.loginStart)
	s.mux.HandleFunc("GET /api/weixin/login/qrcode", s.loginQRCode)
	s.mux.HandleFunc("GET /api/weixin/login/status", s.loginStatus)
	s.mux.HandleFunc("GET /api/weixin/account", s.accountStatus)
	s.mux.HandleFunc("POST /api/weixin/poll/start", s.pollStart)
	s.mux.HandleFunc("POST /api/weixin/poll/stop", s.pollStop)
}

func (s *Server) home(w http.ResponseWriter, _ *http.Request) {
	writeHTML(w, http.StatusOK, homePageHTML)
}

func (s *Server) qrCodeJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(qrCodeJSSource)
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) loginStart(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	session, err := s.deps.LoginService.Start(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessionKey": session.SessionKey,
		"qrCodeUrl":  session.QRCodeURL,
		"expiresAt":  session.ExpiresAt,
	})
}

func (s *Server) loginQRCode(w http.ResponseWriter, r *http.Request) {
	sessionKey := strings.TrimSpace(r.URL.Query().Get("sessionKey"))
	if sessionKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "sessionKey is required"})
		return
	}
	session, ok := s.deps.LoginService.GetSession(sessionKey)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "login session not found or expired"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	client := weixin.NewClient(s.deps.Config.WeixinBaseURL)
	image, err := client.ResolveQRImage(ctx, session.QRCodeURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", weixin.NormalizeImageContentType(image.ContentType))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(image.Data)
}

func (s *Server) loginStatus(w http.ResponseWriter, r *http.Request) {
	sessionKey := strings.TrimSpace(r.URL.Query().Get("sessionKey"))
	if sessionKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "sessionKey is required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	status, err := s.deps.LoginService.Check(ctx, sessionKey)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) accountStatus(w http.ResponseWriter, _ *http.Request) {
	status, err := s.deps.Store.LoadStatus()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	cred, ok, err := s.deps.Store.LoadCredential()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if ok {
		status.Configured = true
		status.AccountID = cred.AccountID
	}
	status.Polling = s.deps.PollManager.Running()
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) pollStart(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.PollManager.Start(context.Background()); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"polling": true})
}

func (s *Server) pollStop(w http.ResponseWriter, _ *http.Request) {
	s.deps.PollManager.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"polling": false})
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeHTML(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
