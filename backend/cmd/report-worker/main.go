package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"anxuncloud/internal/pkg/libreoffice"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	root := os.Getenv("LIBREOFFICE_WORKER_DIR")
	if root == "" {
		root = "/render"
	}
	jobsDir := filepath.Join(root, "jobs")
	if err := os.MkdirAll(jobsDir, 0700); err != nil {
		return err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		entries, err := os.ReadDir(jobsDir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			jobDir := filepath.Join(jobsDir, entry.Name())
			if exists(filepath.Join(jobDir, "output.pdf")) || exists(filepath.Join(jobDir, "error.txt")) {
				continue
			}
			if err := processJob(ctx, jobDir); err != nil {
				message := []byte(err.Error())
				if len(message) > 1024 {
					message = message[:1024]
				}
				_ = os.WriteFile(filepath.Join(jobDir, "error.txt"), message, 0600)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func processJob(ctx context.Context, jobDir string) error {
	maxInput := envBytes("LIBREOFFICE_MAX_INPUT_BYTES", libreoffice.DefaultMaxInputBytes)
	maxOutput := envBytes("LIBREOFFICE_MAX_OUTPUT_BYTES", libreoffice.DefaultMaxOutputBytes)
	input, err := os.ReadFile(filepath.Join(jobDir, "input.docx"))
	if err != nil {
		return err
	}
	timeout := envDuration("LIBREOFFICE_TIMEOUT_SECONDS", 60)
	converter, err := libreoffice.NewExecConverter(os.Getenv("LIBREOFFICE_BINARY"), timeout, 1, maxInput, maxOutput)
	if err != nil {
		return err
	}
	data, err := converter.Convert(ctx, input)
	if err != nil {
		return err
	}
	tempOutput := filepath.Join(jobDir, ".output.tmp")
	if err := os.WriteFile(tempOutput, data, 0600); err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(checkCtx, "pdfinfo", tempOutput).CombinedOutput(); err != nil {
		return fmt.Errorf("PDF 结构校验失败: %w %s", err, output)
	}
	return os.Rename(tempOutput, filepath.Join(jobDir, "output.pdf"))
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func envDuration(key string, fallback int) time.Duration {
	seconds, _ := strconv.Atoi(os.Getenv(key))
	if seconds <= 0 {
		seconds = fallback
	}
	return time.Duration(seconds) * time.Second
}

func envBytes(key string, fallback int64) int64 {
	value, _ := strconv.ParseInt(os.Getenv(key), 10, 64)
	if value <= 0 {
		return fallback
	}
	return value
}
