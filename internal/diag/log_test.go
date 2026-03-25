package diag

import "testing"

func TestRedactSecretMasksLongValues(t *testing.T) {
	raw := "abcdefghijklmnopqrstuvwxyz0123456789"

	got := RedactSecret(raw)

	if got == raw {
		t.Fatalf("RedactSecret() leaked raw secret")
	}
	if got != "abcd...6789(len=36)" {
		t.Fatalf("RedactSecret() = %q", got)
	}
}

func TestPreviewTextCompactsAndTruncates(t *testing.T) {
	raw := "hello   world\nfrom\twechat"

	got := PreviewText(raw, 12)

	if got != "hello world ..." {
		t.Fatalf("PreviewText() = %q", got)
	}
}
