package libreoffice

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"anxuncloud/internal/pkg/types"
)

type WorkerClient struct {
	dir       string
	timeout   time.Duration
	maxInput  int64
	maxOutput int64
}

func NewWorkerClient(dir string, timeout time.Duration, maxInput, maxOutput int64) (*WorkerClient, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("LibreOffice worker 目录不能为空")
	}
	for _, path := range []string{filepath.Join(dir, "jobs"), filepath.Join(dir, "tmp")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			return nil, fmt.Errorf("LibreOffice worker 目录不可用: %w", err)
		}
	}
	probe, err := os.CreateTemp(filepath.Join(dir, "tmp"), ".write-check-")
	if err != nil {
		return nil, fmt.Errorf("LibreOffice worker 目录不可写: %w", err)
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	if maxInput <= 0 {
		maxInput = DefaultMaxInputBytes
	}
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutputBytes
	}
	return &WorkerClient{dir: dir, timeout: timeout, maxInput: maxInput, maxOutput: maxOutput}, nil
}

func (c *WorkerClient) Convert(ctx context.Context, docx []byte) ([]byte, error) {
	if err := validateInput(docx, c.maxInput); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	jobID := types.NewID()
	tempDir := filepath.Join(c.dir, "tmp", jobID)
	jobDir := filepath.Join(c.dir, "jobs", jobID)
	if err := os.Mkdir(tempDir, 0700); err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	defer os.RemoveAll(jobDir)
	if err := os.WriteFile(filepath.Join(tempDir, "input.docx"), docx, 0600); err != nil {
		return nil, err
	}
	if err := os.Rename(tempDir, jobDir); err != nil {
		return nil, fmt.Errorf("转换任务发布失败: %w", err)
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		output, err := os.ReadFile(filepath.Join(jobDir, "error.txt"))
		if err == nil {
			message := strings.TrimSpace(string(output))
			if len(message) > 512 {
				message = message[:512]
			}
			return nil, fmt.Errorf("LibreOffice worker 转换失败: %s", message)
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		file, err := os.Open(filepath.Join(jobDir, "output.pdf"))
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(file, c.maxOutput+1))
			_ = file.Close()
			if readErr != nil {
				return nil, readErr
			}
			if err := ValidatePDF(data, c.maxOutput); err != nil {
				return nil, err
			}
			return data, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("LibreOffice worker 转换超时: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}
