package libreoffice

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestExecConverterWithControlledCommand(t *testing.T) {
	dir := t.TempDir()
	name := "fake-soffice.sh"
	content := "#!/bin/sh\nwhile [ $# -gt 0 ]; do if [ \"$1\" = \"--outdir\" ]; then OUT=\"$2\"; fi; shift; done\nprintf '%%PDF-1.7\\nbody\\n%%%%EOF\\n' > \"$OUT/input.pdf\"\n"
	if runtime.GOOS == "windows" {
		name = "fake-soffice.cmd"
		content = "@echo off\r\n:loop\r\nif \"%~1\"==\"\" goto done\r\nif \"%~1\"==\"--outdir\" set \"OUT=%~2\"\r\nshift\r\ngoto loop\r\n:done\r\necho %%PDF-1.7> \"%OUT%\\input.pdf\"\r\necho body>> \"%OUT%\\input.pdf\"\r\necho %%%%EOF>> \"%OUT%\\input.pdf\"\r\n"
	}
	binary := filepath.Join(dir, name)
	if err := os.WriteFile(binary, []byte(content), 0700); err != nil {
		t.Fatal(err)
	}
	converter, err := NewExecConverter(binary, 2*time.Second, 1, 1024, 1024)
	if err != nil {
		t.Fatal(err)
	}
	data, err := converter.Convert(context.Background(), []byte("PK\x03\x04test"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidatePDF(data, 1024); err != nil {
		t.Fatal(err)
	}
}

func TestExecConverterRequiresBinary(t *testing.T) {
	if _, err := NewExecConverter(filepath.Join(t.TempDir(), "missing"), time.Second, 1, 1024, 1024); err == nil {
		t.Fatal("缺少 LibreOffice 时不应创建转换器")
	}
}
