package delete

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"repomop/internal/scanner"
	"repomop/internal/testutil"
)

func TestArtifactsDeletesSelectedDirectories(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")

	testutil.MkdirAll(t, one)
	testutil.MkdirAll(t, two)
	testutil.WriteFileSized(t, filepath.Join(one, "a.bin"), 10)
	testutil.WriteFileSized(t, filepath.Join(two, "b.bin"), 20)

	selected := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: one, ProjectRoot: root, SizeBytes: 10},
	}

	result := Artifacts(context.Background(), selected)
	if len(result.Deleted) != 1 {
		t.Fatalf("expected 1 deleted artifact, got %d", len(result.Deleted))
	}
	if result.FreedBytes != 10 {
		t.Fatalf("expected freed bytes 10, got %d", result.FreedBytes)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}

	if _, err := os.Stat(one); !os.IsNotExist(err) {
		t.Fatalf("expected first artifact deleted, stat err=%v", err)
	}
	if _, err := os.Stat(two); err != nil {
		t.Fatalf("expected second artifact untouched, stat err=%v", err)
	}
}

func TestArtifactsDeletesMultiple(t *testing.T) {
	root := t.TempDir()
	one := filepath.Join(root, "one")
	two := filepath.Join(root, "two")

	testutil.MkdirAll(t, one)
	testutil.MkdirAll(t, two)
	testutil.WriteFileSized(t, filepath.Join(one, "a.bin"), 10)
	testutil.WriteFileSized(t, filepath.Join(two, "b.bin"), 20)

	selected := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: one, ProjectRoot: root, SizeBytes: 10},
		{Kind: scanner.ArtifactNodeModule, Path: two, ProjectRoot: root, SizeBytes: 20},
	}

	result := Artifacts(context.Background(), selected)
	if len(result.Deleted) != 2 {
		t.Fatalf("expected 2 deleted artifacts, got %d", len(result.Deleted))
	}
	if result.FreedBytes != 30 {
		t.Fatalf("expected freed bytes 30, got %d", result.FreedBytes)
	}
	if len(result.Errors) != 0 {
		t.Fatalf("expected no errors, got %d", len(result.Errors))
	}
}

func TestArtifactsEmptySlice(t *testing.T) {
	result := Artifacts(context.Background(), []scanner.Artifact{})
	if len(result.Deleted) != 0 {
		t.Fatalf("expected 0 deleted, got %d", len(result.Deleted))
	}
	if result.FreedBytes != 0 {
		t.Fatalf("expected 0 freed bytes, got %d", result.FreedBytes)
	}
}

func TestArtifactsCollectsErrorsForNonexistentPath(t *testing.T) {
	selected := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: "/nonexistent/path/that/does/not/exist", SizeBytes: 100},
	}

	result := Artifacts(context.Background(), selected)

	// os.RemoveAll does not error on nonexistent paths, so it should be "deleted"
	if len(result.Deleted) != 1 {
		t.Fatalf("expected 1 deleted (RemoveAll succeeds for nonexistent), got %d deleted, %d errors", len(result.Deleted), len(result.Errors))
	}
}

func TestArtifactsCollectsRemovalErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not supported on windows")
	}

	root := t.TempDir()
	protected := filepath.Join(root, "protected")
	child := filepath.Join(protected, "child")
	testutil.MkdirAll(t, child)
	testutil.WriteFileSized(t, filepath.Join(child, "f.bin"), 10)

	// Make parent non-writable so child can't be removed
	if err := os.Chmod(protected, 0o555); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(protected, 0o755) })

	selected := []scanner.Artifact{
		{Kind: scanner.ArtifactNodeModule, Path: child, ProjectRoot: root, SizeBytes: 10},
	}

	result := Artifacts(context.Background(), selected)
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d errors and %d deleted", len(result.Errors), len(result.Deleted))
	}
	if result.FreedBytes != 0 {
		t.Fatalf("expected 0 freed bytes on error, got %d", result.FreedBytes)
	}
}
