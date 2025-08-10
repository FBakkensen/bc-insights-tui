package debugdump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePath_Defaults(t *testing.T) {
	p, err := ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath(\"\") error: %v", err)
	}
	if !strings.HasSuffix(p, filepath.FromSlash("logs/appinsights-raw.yaml")) {
		t.Fatalf("unexpected default path: %s", p)
	}
	p2, err := ResolvePath("cap")
	if err != nil {
		t.Fatalf("ResolvePath(\"cap\") error: %v", err)
	}
	if !strings.HasSuffix(p2, filepath.FromSlash("logs/cap.yaml")) {
		t.Fatalf("unexpected path for bare filename: %s", p2)
	}
}

func TestResolvePath_KeepsExtension(t *testing.T) {
	p, err := ResolvePath("cap.log")
	if err != nil {
		t.Fatalf("ResolvePath(\"cap.log\") error: %v", err)
	}
	if !strings.HasSuffix(p, filepath.FromSlash("logs/cap.log")) {
		t.Fatalf("expected logs/cap.log, got %s", p)
	}
	p2, err := ResolvePath("logs/out.yml")
	if err != nil {
		t.Fatalf("ResolvePath(\"logs/out.yml\") error: %v", err)
	}
	if !strings.HasSuffix(p2, filepath.FromSlash("logs/out.yml")) {
		t.Fatalf("expected logs/out.yml, got %s", p2)
	}
}

func TestWriteYAMLAtomic_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cap.yaml")
	doc1 := AIRawCapture{Version: 1}
	if err := WriteAIRawFull(path, doc1); err != nil {
		t.Fatalf("write1: %v", err)
	}
	st1, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat1: %v", err)
	}
	doc2 := AIRawCapture{Version: 2}
	err = WriteAIRawFull(path, doc2)
	if err != nil {
		t.Fatalf("write2: %v", err)
	}
	st2, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat2: %v", err)
	}
	if st2.ModTime().Before(st1.ModTime()) {
		t.Fatalf("expected newer modtime after overwrite")
	}
}
