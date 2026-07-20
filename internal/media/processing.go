package media

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Job struct {
	mu                   sync.Mutex
	ID                   string  `json:"id"`
	Status               string  `json:"status"`
	Progress             int     `json:"progress"`
	DurationSec          float64 `json:"-"`
	Err                  string  `json:"error,omitempty"`
	TotalBytes           int64   `json:"-"`
	ReceivedBytes        int64   `json:"-"`
	EstimatedMinutesLeft float64 `json:"estimated_minutes_left,omitempty"`
	startedAt            time.Time
}

func (j *Job) setDuration(d float64) {
	j.mu.Lock()

	defer j.mu.Unlock()

	j.DurationSec = d
}

func (j *Job) setProgress(p int) {
	j.mu.Lock()

	defer j.mu.Unlock()

	if p > 100 {
		p = 100
	}
	j.Progress = p
	if j.Status == "pending" {
		j.Status = "processing"
	}
	if j.Progress == 100 {
		j.Status = "completed"
	}
}

func (j *Job) setStatus(s string) {
	j.mu.Lock()

	defer j.mu.Unlock()

	j.Status = s
}

func (j *Job) setError(err string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Status = "error"
	j.Err = err
}

// addReceived records n more uploaded bytes and refreshes progress/ETA.
func (j *Job) addReceived(n int64) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.ReceivedBytes += n
	if j.Status == "pending" {
		j.Status = "uploading"
	}
	if j.TotalBytes <= 0 {
		return
	}

	pct := int(float64(j.ReceivedBytes) / float64(j.TotalBytes) * 100)
	if pct > 100 {
		pct = 100
	}
	j.Progress = pct

	if elapsed := time.Since(j.startedAt).Seconds(); elapsed > 0 {
		rate := float64(j.ReceivedBytes) / elapsed // bytes/sec
		j.EstimatedMinutesLeft = float64(j.TotalBytes-j.ReceivedBytes) / rate / 60
	}
}

func (j *Job) finishUpload() {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.Progress = 100
	j.Status = "completed"
	j.EstimatedMinutesLeft = 0
}

// JobSnapshot is a lock-free copy of a Job's fields, safe to pass around.
type JobSnapshot struct {
	ID                   string  `json:"id"`
	Status               string  `json:"status"`
	Progress             int     `json:"progress"`
	DurationSec          float64 `json:"-"`
	Err                  string  `json:"error,omitempty"`
	TotalBytes           int64   `json:"-"`
	ReceivedBytes        int64   `json:"-"`
	EstimatedMinutesLeft float64 `json:"estimated_minutes_left,omitempty"`
}

// Snapshot returns a copy of the job's fields safe to read without holding
// the lock or copying it.
func (j *Job) Snapshot() JobSnapshot {
	j.mu.Lock()

	defer j.mu.Unlock()

	return JobSnapshot{
		ID:                   j.ID,
		Status:               j.Status,
		Progress:             j.Progress,
		DurationSec:          j.DurationSec,
		Err:                  j.Err,
		TotalBytes:           j.TotalBytes,
		ReceivedBytes:        j.ReceivedBytes,
		EstimatedMinutesLeft: j.EstimatedMinutesLeft,
	}
}

type Store struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func newStore() *Store {
	return &Store{
		jobs: make(map[string]*Job),
	}
}

func (s *Store) create(id string, totalBytes int64) *Job {
	s.mu.Lock()
	defer s.mu.Unlock()

	job := &Job{ID: id, Status: "pending", TotalBytes: totalBytes, startedAt: time.Now()}
	s.jobs[id] = job
	return job
}

func (s *Store) get(id string) (*Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, ok := s.jobs[id]
	return job, ok
}

// jobs is an in-memory registry of in-flight upload/transcode jobs.
// ponytail: single-process map; move to a Redis-backed Store if this needs
// to survive restarts or be visible across multiple server instances.
var jobs = newStore()

// CreateJob registers a new job for tracking an upload or transcode. Pass
// totalBytes > 0 for byte-progress tracking (uploads); 0 for ffmpeg-progress
// tracking (transcodes), which drives progress via watchFFmpegProgress instead.
func CreateJob(id string, totalBytes int64) *Job {
	return jobs.create(id, totalBytes)
}

// GetJob looks up a previously registered job by id.
func GetJob(id string) (*Job, bool) {
	return jobs.get(id)
}

// VideoInfo is the subset of ffprobe metadata needed to drive a transcode.
type VideoInfo struct {
	Width       int
	Height      int
	DurationSec float64
}

// Probe extracts width/height/duration from the video at path via ffprobe.
func Probe(ctx context.Context, path string) (VideoInfo, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=width,height:format=duration",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return VideoInfo{}, fmt.Errorf("ffprobe: %w", err)
	}

	var parsed struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return VideoInfo{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return VideoInfo{}, fmt.Errorf("no video stream in %s", path)
	}

	duration, _ := strconv.ParseFloat(parsed.Format.Duration, 64)

	return VideoInfo{
		Width:       parsed.Streams[0].Width,
		Height:      parsed.Streams[0].Height,
		DurationSec: duration,
	}, nil
}

// watchFFmpegProgress parses ffmpeg's "-progress pipe:1" stream, updating
// job's Progress as out_time_ms advances against durationSec. ffmpeg's key
// is named out_time_ms but actually reports microseconds.
func watchFFmpegProgress(job *Job, stdout io.Reader, durationSec float64) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		key, val, ok := strings.Cut(scanner.Text(), "=")
		if !ok || key != "out_time_ms" || durationSec <= 0 {
			continue
		}
		us, err := strconv.ParseFloat(val, 64)
		if err != nil {
			continue
		}
		job.setProgress(int(us / 1e6 / durationSec * 100))
	}
}

// Transcode converts srcPath into an mp4 at dstPath, scaling to targetHeight.
// The output height is capped at the source's own height so ffmpeg never
// upscales past the original quality; pass targetHeight <= 0 to keep the
// source resolution as-is.
func Transcode(ctx context.Context, job *Job, srcPath, dstPath string, targetHeight int) error {
	info, err := Probe(ctx, srcPath)
	if err != nil {
		job.setError(err.Error())
		return err
	}
	job.setDuration(info.DurationSec)

	height := targetHeight
	if height <= 0 || height > info.Height {
		height = info.Height
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", srcPath,
		"-vf", fmt.Sprintf("scale=-2:%d", height),
		"-progress", "pipe:1",
		"-nostats",
		dstPath,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.setError(err.Error())
		return err
	}
	if err := cmd.Start(); err != nil {
		job.setError(err.Error())
		return err
	}

	watchFFmpegProgress(job, stdout, info.DurationSec)

	if err := cmd.Wait(); err != nil {
		job.setError(err.Error())
		return err
	}

	job.setProgress(100)
	return nil
}

// TranscodeHLS converts srcPath into an HLS rendition (playlist.m3u8 + .ts
// segments) under outDir, scaled to targetHeight, reporting progress on job
// the same way Transcode does. Caller is responsible for clamping
// targetHeight to the source's own height so ffmpeg never upscales.
func TranscodeHLS(ctx context.Context, job *Job, srcPath, outDir string, targetHeight int) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		job.setError(err.Error())
		return err
	}

	info, err := Probe(ctx, srcPath)
	if err != nil {
		job.setError(err.Error())
		return err
	}
	job.setDuration(info.DurationSec)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-i", srcPath,
		"-vf", fmt.Sprintf("scale=-2:%d", targetHeight),
		"-c:v", "libx264",
		"-c:a", "aac",
		"-hls_time", "6",
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(outDir, "segment_%03d.ts"),
		"-progress", "pipe:1",
		"-nostats",
		filepath.Join(outDir, "playlist.m3u8"),
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.setError(err.Error())
		return err
	}
	if err := cmd.Start(); err != nil {
		job.setError(err.Error())
		return err
	}

	watchFFmpegProgress(job, stdout, info.DurationSec)

	if err := cmd.Wait(); err != nil {
		wrapped := fmt.Errorf("ffmpeg hls: %w: %s", err, stderr.String())
		job.setError(wrapped.Error())
		return wrapped
	}

	job.setProgress(100)
	return nil
}

type progressWriter struct{ job *Job }

func (w progressWriter) Write(p []byte) (int, error) {
	w.job.addReceived(int64(len(p)))
	return len(p), nil
}

// SaveUpload streams r to dst, updating job's Progress and
// EstimatedMinutesLeft as bytes arrive (job.TotalBytes must already be set,
// e.g. via CreateJob).
func SaveUpload(job *Job, dst string, r io.Reader) error {
	f, err := os.Create(dst)
	if err != nil {
		job.setError(err.Error())
		return err
	}
	defer f.Close()

	if _, err := io.Copy(io.MultiWriter(f, progressWriter{job}), r); err != nil {
		job.setError(err.Error())
		return err
	}

	job.finishUpload()
	return nil
}
