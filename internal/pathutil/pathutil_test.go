package pathutil

import (
	"path/filepath"
	"testing"
)

func TestRelativePathOrSelfEmptyRoot(t *testing.T) {
	got := RelativePathOrSelf("", "/some/path")
	if got != "/some/path" {
		t.Fatalf("expected original path, got %q", got)
	}
}

func TestRelativePathOrSelfEmptyPath(t *testing.T) {
	got := RelativePathOrSelf("/root", "")
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestRelativePathOrSelfSamePath(t *testing.T) {
	got := RelativePathOrSelf("/root", "/root")
	if got != "/root" {
		t.Fatalf("expected original path when equal to root, got %q", got)
	}
}

func TestRelativePathOrSelfReturnsRelative(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	path := filepath.Join(root, "src", "main.go")
	got := RelativePathOrSelf(root, path)
	expected := filepath.Join("src", "main.go")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestRelativePathOrSelfNestedChild(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "home", "user", "project")
	path := filepath.Join(root, "a", "b", "c")
	got := RelativePathOrSelf(root, path)
	expected := filepath.Join("a", "b", "c")
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}
