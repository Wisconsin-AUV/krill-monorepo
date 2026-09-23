package ingest

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type probeResult struct {
	FPS        float64
	DurationMs int64
}

func probe(ctx context.Context, path string) (probeResult, error) {
	//nolint:gosec // path is a file in our own temp dir
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=avg_frame_rate,r_frame_rate:format=duration",
		"-of", "json",
		path,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return probeResult{}, fmt.Errorf("ffprobe: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var parsed struct {
		Streams []struct {
			AvgFrameRate string `json:"avg_frame_rate"`
			RFrameRate   string `json:"r_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return probeResult{}, fmt.Errorf("parse ffprobe output: %w", err)
	}
	if len(parsed.Streams) == 0 {
		return probeResult{}, fmt.Errorf("no video stream found")
	}

	fps := parseRate(parsed.Streams[0].AvgFrameRate)
	if fps <= 0 {
		fps = parseRate(parsed.Streams[0].RFrameRate)
	}
	if fps <= 0 {
		return probeResult{}, fmt.Errorf("could not read frame rate")
	}
	duration, _ := strconv.ParseFloat(parsed.Format.Duration, 64)
	return probeResult{FPS: fps, DurationMs: int64(duration * 1000)}, nil
}

func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	n, err1 := strconv.ParseFloat(num, 64)
	d, err2 := strconv.ParseFloat(den, 64)
	if err1 != nil || err2 != nil || d == 0 {
		return 0
	}
	return n / d
}

// extractFrames writes every frame (or fps frames per second when fps > 0) to
// dir as zero-based numbered JPEGs, reporting the running frame count.
func extractFrames(ctx context.Context, src, dir string, fps float64, onFrame func(int)) error {
	args := []string{"-nostdin", "-v", "error", "-i", src}
	if fps > 0 {
		args = append(args, "-vf", fmt.Sprintf("fps=%g", fps))
	}
	args = append(args,
		"-fps_mode", "passthrough",
		"-q:v", "2",
		"-start_number", "0",
		"-progress", "pipe:1",
		filepath.Join(dir, "%06d.jpg"),
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...) //nolint:gosec // args are built here, not from user input
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("ffmpeg stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start ffmpeg: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if v, ok := strings.CutPrefix(scanner.Text(), "frame="); ok {
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				onFrame(n)
			}
		}
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
