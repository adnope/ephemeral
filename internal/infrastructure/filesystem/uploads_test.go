package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveTreeRemovesSafeUploadSubtree(t *testing.T) {
	t.Parallel()

	storage := NewUploadStorage(t.TempDir())
	hlsDir := filepath.Join(storage.uploadDir, "hls", "sample")
	if err := os.MkdirAll(hlsDir, 0o755); err != nil {
		t.Fatalf("mkdir hls dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hlsDir, "segment_00000.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}

	if err := storage.RemoveTree("hls/sample"); err != nil {
		t.Fatalf("RemoveTree(): %v", err)
	}
	if _, err := os.Stat(hlsDir); !os.IsNotExist(err) {
		t.Fatalf("hls dir stat error = %v, want not exist", err)
	}
}

func TestRemoveTreeRejectsUnsafePath(t *testing.T) {
	t.Parallel()

	storage := NewUploadStorage(t.TempDir())
	if err := storage.RemoveTree("../outside"); err == nil {
		t.Fatal("RemoveTree() error = nil, want unsafe path error")
	}
}
