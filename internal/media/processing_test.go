package media

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
}

// makeTestVideo generates a tiny synthetic mp4 (640x360) for use as a fixture.
func makeTestVideo(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "src.mp4")
	cmd := exec.Command("ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=640x360:duration=1:rate=10",
		"-c:v", "libx264",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v\n%s", err, out)
	}
	return path
}

func TestTranscodeNeverUpscales(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := makeTestVideo(t, dir)
	dst := filepath.Join(dir, "out.mp4")

	job := &Job{ID: "t1", Status: "pending"}
	if err := Transcode(context.Background(), job, src, dst, 720); err != nil {
		t.Fatalf("transcode: %v", err)
	}

	info, err := Probe(context.Background(), dst)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if info.Height != 360 {
		t.Fatalf("expected clamp to source height 360, got %d", info.Height)
	}

	snap := job.Snapshot()
	if snap.Status != "completed" || snap.Progress != 100 {
		t.Fatalf("expected completed job at 100%%, got %+v", snap)
	}
}

func TestTranscodeDownscales(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	src := makeTestVideo(t, dir)
	dst := filepath.Join(dir, "out.mp4")

	job := &Job{ID: "t2", Status: "pending"}
	if err := Transcode(context.Background(), job, src, dst, 180); err != nil {
		t.Fatalf("transcode: %v", err)
	}

	info, err := Probe(context.Background(), dst)
	if err != nil {
		t.Fatalf("probe output: %v", err)
	}
	if info.Height != 180 {
		t.Fatalf("expected requested height 180, got %d", info.Height)
	}
}
