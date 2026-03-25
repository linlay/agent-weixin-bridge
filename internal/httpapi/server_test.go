package httpapi

import (
	"agent-weixin-bridge/internal/config"
	"agent-weixin-bridge/internal/state"
	"agent-weixin-bridge/internal/weixin"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomePageServesHTML(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
	body := rec.Body.String()
	for _, marker := range []string{
		"微信桥接控制台",
		"生成二维码",
		"开始轮询",
		"停止轮询",
		"二维码源：",
		"/assets/qrcode.js",
		"/api/weixin/login/start",
		"/api/weixin/account",
		"id=\"qrCanvas\"",
		"generated-from-url",
		"direct-image",
	} {
		if !strings.Contains(body, marker) {
			t.Fatalf("home page missing marker %q", marker)
		}
	}
}

func TestQRCodeAssetServesJavaScript(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/assets/qrcode.js", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	contentType := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/javascript") {
		t.Fatalf("Content-Type = %q, want javascript", contentType)
	}
	if body := rec.Body.String(); !strings.Contains(body, "QR Code Generator for JavaScript") {
		t.Fatalf("asset body missing qrcode marker")
	}
}

func TestHealthzStillServesJSON(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != "{\"ok\":true}" {
		t.Fatalf("body = %q, want %q", got, "{\"ok\":true}")
	}
}

func TestLoginQRCodeRequiresSessionKey(t *testing.T) {
	server := NewServer(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/api/weixin/login/qrcode", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestLoginQRCodeReturnsImage(t *testing.T) {
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-token",
				"qrcode_img_content": upstream.URL + "/qrcode.png",
			})
		case "/qrcode.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(samplePNGHTTPAPI)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	store, err := state.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	loginService := weixin.NewLoginService(weixin.NewClient(upstream.URL), store, "3")
	session, err := loginService.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	server := NewServer(Dependencies{
		Config:       config.Config{WeixinBaseURL: upstream.URL},
		Store:        store,
		LoginService: loginService,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/weixin/login/qrcode?sessionKey="+session.SessionKey, nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("Content-Type = %q, want %q", got, "image/png")
	}
	if rec.Body.Len() == 0 {
		t.Fatalf("expected image body")
	}
}

func TestLoginQRCodeReturnsNotFoundForMissingSession(t *testing.T) {
	store, err := state.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	server := NewServer(Dependencies{
		Config:       config.Config{WeixinBaseURL: "http://127.0.0.1"},
		Store:        store,
		LoginService: weixin.NewLoginService(weixin.NewClient("http://127.0.0.1"), store, "3"),
	})
	req := httptest.NewRequest(http.MethodGet, "/api/weixin/login/qrcode?sessionKey=missing", nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestLoginQRCodeRejectsHTMLResponse(t *testing.T) {
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ilink/bot/get_bot_qrcode":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"qrcode":             "qr-token",
				"qrcode_img_content": upstream.URL + "/qrcode.html",
			})
		case "/qrcode.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><body>loading</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	store, err := state.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}
	loginService := weixin.NewLoginService(weixin.NewClient(upstream.URL), store, "3")
	session, err := loginService.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	server := NewServer(Dependencies{
		Config:       config.Config{WeixinBaseURL: upstream.URL},
		Store:        store,
		LoginService: loginService,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/weixin/login/qrcode?sessionKey="+session.SessionKey, nil)
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rec.Body.String(), "non-image content-type") {
		t.Fatalf("body = %q, want non-image error", rec.Body.String())
	}
}

var samplePNGHTTPAPI = mustDecodePNG()

func mustDecodePNG() []byte {
	data, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGNgYGD4DwABBAEAQP6nWQAAAABJRU5ErkJggg==")
	if err != nil {
		panic(err)
	}
	return data
}
