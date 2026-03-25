package weixin

import (
	"encoding/base64"
	"testing"
)

func TestNormalizeQRCodeValueTrimsWhitespace(t *testing.T) {
	raw := "  https://liteapp.weixin.qq.com/q/demo?qrcode=abc  \n"

	got := NormalizeQRCodeValue(raw)

	want := "https://liteapp.weixin.qq.com/q/demo?qrcode=abc"
	if got != want {
		t.Fatalf("NormalizeQRCodeValue() = %q, want %q", got, want)
	}
}

func TestClassifyQRCodeValue(t *testing.T) {
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(samplePNG)
	base64Image := base64.StdEncoding.EncodeToString(samplePNG)

	tests := []struct {
		name string
		raw  string
		want QRCodeValueKind
		lite bool
	}{
		{name: "empty", raw: "   ", want: QRCodeValueEmpty},
		{name: "data url", raw: dataURL, want: QRCodeValueDataURL},
		{name: "base64 image", raw: base64Image, want: QRCodeValueBase64Image},
		{name: "http image", raw: "https://example.com/qrcode.png", want: QRCodeValueHTTPURL},
		{name: "liteapp page", raw: "https://liteapp.weixin.qq.com/q/7GiQu1?qrcode=293b036c06032776f339f34ed72be9f8&bot_type=3", want: QRCodeValueLiteAppURL, lite: true},
		{name: "unsupported", raw: "not-a-qrcode", want: QRCodeValueUnsupported},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyQRCodeValue(tt.raw)
			if got != tt.want {
				t.Fatalf("ClassifyQRCodeValue() = %q, want %q", got, tt.want)
			}
			if IsLiteAppQRCodeURL(tt.raw) != tt.lite {
				t.Fatalf("IsLiteAppQRCodeURL() = %v, want %v", IsLiteAppQRCodeURL(tt.raw), tt.lite)
			}
		})
	}
}
