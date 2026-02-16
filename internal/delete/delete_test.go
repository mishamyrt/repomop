package delete

import (
	"os"
	"path/filepath"
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

	result := Artifacts(selected)
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

