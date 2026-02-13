package size

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoriesCalculatesNestedSize(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")

	mustMkdirAll(t, filepath.Join(dirA, "nested"))
	mustMkdirAll(t, dirB)

	mustWriteFileSized(t, filepath.Join(dirA, "nested", "f1.bin"), 10)
	mustWriteFileSized(t, filepath.Join(dirA, "nested", "f2.bin"), 15)
	mustWriteFileSized(t, filepath.Join(dirB, "f3.bin"), 8)

	sizes, errs := Directories([]string{dirA, dirB}, 2)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[dirA] != 25 {
		t.Fatalf("expected dirA size 25, got %d", sizes[dirA])
	}
	if sizes[dirB] != 8 {
		t.Fatalf("expected dirB size 8, got %d", sizes[dirB])
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFileSized(t *testing.T, path string, size int) {
	t.Helper()
	content := make([]byte, size)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
