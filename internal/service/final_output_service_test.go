package service

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAssembleVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg is required")
	}
	dir := t.TempDir()
	t.Setenv("DEMO_ASSET_DIR", dir)
	t.Setenv("ENV", "development")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	input := filepath.Join(dir, "sample.mp4")
	if err := exec.CommandContext(ctx, "ffmpeg", "-v", "error", "-f", "lavfi", "-i", "color=c=green:s=320x180:d=0.5", "-c:v", "libx264", "-pix_fmt", "yuv420p", input).Run(); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := AssembleVideo(ctx, []string{"/demo-assets/sample.mp4", "/demo-assets/sample.mp4"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "default=noprint_wrappers=1:nokey=1", path).Output()
	if err != nil {
		t.Fatal(err)
	}
	duration, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil || duration < 0.9 || duration > 1.4 {
		t.Fatal("clips were not combined", duration, err)
	}
	cleanup()
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("temporary media retained")
	}
	_, cleanup, err = AssembleVideo(ctx, []string{"/demo-assets/../sample.mp4"})
	cleanup()
	if err == nil {
		t.Fatal("path traversal accepted")
	}
}
