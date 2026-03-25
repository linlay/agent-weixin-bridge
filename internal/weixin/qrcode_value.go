package weixin

import (
	"net/url"
	"strings"
)

type QRCodeValueKind string

const (
	QRCodeValueEmpty       QRCodeValueKind = "empty"
	QRCodeValueDataURL     QRCodeValueKind = "data-url"
	QRCodeValueBase64Image QRCodeValueKind = "base64-image"
	QRCodeValueHTTPURL     QRCodeValueKind = "http-url"
	QRCodeValueLiteAppURL  QRCodeValueKind = "liteapp-url"
	QRCodeValueUnsupported QRCodeValueKind = "unsupported"
)

func NormalizeQRCodeValue(raw string) string {
	return strings.TrimSpace(raw)
}

func ClassifyQRCodeValue(raw string) QRCodeValueKind {
	value := NormalizeQRCodeValue(raw)
	switch {
	case value == "":
		return QRCodeValueEmpty
	case strings.HasPrefix(value, "data:"):
		return QRCodeValueDataURL
	case looksLikeBase64Image(value):
		return QRCodeValueBase64Image
	case isHTTPQRCodeValue(value):
		if IsLiteAppQRCodeURL(value) {
			return QRCodeValueLiteAppURL
		}
		return QRCodeValueHTTPURL
	default:
		return QRCodeValueUnsupported
	}
}

func IsLiteAppQRCodeURL(raw string) bool {
	value := NormalizeQRCodeValue(raw)
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), "liteapp.weixin.qq.com") && strings.HasPrefix(parsed.EscapedPath(), "/q/")
}

func isHTTPQRCodeValue(raw string) bool {
	value := NormalizeQRCodeValue(raw)
	if value == "" {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}
