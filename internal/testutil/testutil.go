package testutil

import (
	"os"
	"testing"
)

// MkdirAll creates a directory and all parents, failing the test on error.
func MkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// WriteFile writes a string to a file, failing the test on error.
func WriteFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// WriteFileSized creates a file filled with zero bytes of the given size.
func WriteFileSized(t *testing.T, path string, size int) {
	t.Helper()
	content := make([]byte, size)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// Symlink creates a symbolic link, failing the test on error.
func Symlink(t *testing.T, target string, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("symlink %s -> %s: %v", path, target, err)
	}
}
