package web

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRendererParsesTemplates(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine current file path")
	}

	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := os.Chdir(root); err != nil {
		t.Fatalf("os.Chdir(%q): %v", root, err)
	}

	if renderer := NewRenderer(); renderer == nil {
		t.Fatal("NewRenderer returned nil")
	}
}
