package libreoffice

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ExecConverter struct {
	binary    string
	timeout   time.Duration
	maxInput  int64
	maxOutput int64
	semaphore chan struct{}
}

func NewExecConverter(binary string, timeout time.Duration, maxConcurrent int, maxInput, maxOutput int64) (*ExecConverter, error) {
	if binary == "" {
		binary = "soffice"
	}
	resolved, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("LibreOffice 不可用: %w", err)
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}
	if maxInput <= 0 {
		maxInput = DefaultMaxInputBytes
	}
	if maxOutput <= 0 {
		maxOutput = DefaultMaxOutputBytes
	}
	return &ExecConverter{binary: resolved, timeout: timeout, maxInput: maxInput, maxOutput: maxOutput, semaphore: make(chan struct{}, maxConcurrent)}, nil
}

func (c *ExecConverter) Convert(ctx context.Context, docx []byte) ([]byte, error) {
	if err := validateInput(docx, c.maxInput); err != nil {
		return nil, err
	}
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	workDir, err := os.MkdirTemp("", "report-libreoffice-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workDir)
	input := filepath.Join(workDir, "input.docx")
	if err := os.WriteFile(input, docx, 0600); err != nil {
		return nil, err
	}
	outputDir := filepath.Join(workDir, "output")
	profileDir := filepath.Join(workDir, "profile")
	for _, dir := range []string{outputDir, profileDir} {
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, err
		}
	}
	profileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(profileDir)}).String()
	command := exec.CommandContext(ctx, c.binary,
		"-env:UserInstallation="+profileURL,
		"--headless", "--nologo", "--nodefault", "--nofirststartwizard", "--nolockcheck",
		"--convert-to", "pdf", "--outdir", outputDir, input,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if len(message) > 512 {
			message = message[:512]
		}
		return nil, fmt.Errorf("LibreOffice 转换失败: %w %s", err, message)
	}
	file, err := os.Open(filepath.Join(outputDir, "input.pdf"))
	if err != nil {
		return nil, fmt.Errorf("LibreOffice 未生成 PDF: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, c.maxOutput+1))
	if err != nil {
		return nil, err
	}
	if err := ValidatePDF(data, c.maxOutput); err != nil {
		return nil, err
	}
	return data, nil
}
