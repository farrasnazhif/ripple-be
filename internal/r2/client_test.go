package r2

import (
	"context"
	"strings"
	"testing"
)

func TestPresignAttachmentURLs(t *testing.T) {
	c, err := New("account", "access", "secret", "briefs")
	if err != nil {
		t.Fatal(err)
	}
	put, err := c.PresignUpload(context.Background(), "briefs/one/file", "image/png")
	if err != nil || !strings.Contains(put, "X-Amz-Signature=") || !strings.Contains(put, "account.r2.cloudflarestorage.com") {
		t.Fatalf("invalid PUT URL: %v", err)
	}
	get, err := c.PresignDownload(context.Background(), "briefs/one/file")
	if err != nil || !strings.Contains(get, "X-Amz-Signature=") {
		t.Fatalf("invalid GET URL: %v", err)
	}
}
