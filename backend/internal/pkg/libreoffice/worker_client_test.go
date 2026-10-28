package libreoffice

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkerClientProtocol(t *testing.T) {
	root := t.TempDir()
	client, err := NewWorkerClient(root, 2*time.Second, 1024, 1024)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		jobs := filepath.Join(root, "jobs")
		for {
			entries, err := os.ReadDir(jobs)
			if err == nil && len(entries) == 1 {
				jobDir := filepath.Join(jobs, entries[0].Name())
				done <- os.WriteFile(filepath.Join(jobDir, "output.pdf"), []byte("%PDF-1.7\nworker\n%%EOF\n"), 0600)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	data, err := client.Convert(context.Background(), []byte("PK\x03\x04test"))
	if err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := ValidatePDF(data, 1024); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerClientReturnsWorkerError(t *testing.T) {
	root := t.TempDir()
	client, err := NewWorkerClient(root, 2*time.Second, 1024, 1024)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		jobs := filepath.Join(root, "jobs")
		for {
			entries, err := os.ReadDir(jobs)
			if err == nil && len(entries) == 1 {
				_ = os.WriteFile(filepath.Join(jobs, entries[0].Name(), "error.txt"), []byte("controlled failure"), 0600)
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	if _, err := client.Convert(context.Background(), []byte("PK\x03\x04test")); err == nil {
		t.Fatal("worker 错误不应被忽略")
	}
}
