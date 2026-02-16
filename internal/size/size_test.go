package size

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"repomop/internal/testutil"
)

func TestDirectoriesCalculatesNestedSize(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "a")
	dirB := filepath.Join(root, "b")

	testutil.MkdirAll(t, filepath.Join(dirA, "nested"))
	testutil.MkdirAll(t, dirB)

	testutil.WriteFileSized(t, filepath.Join(dirA, "nested", "f1.bin"), 10)
	testutil.WriteFileSized(t, filepath.Join(dirA, "nested", "f2.bin"), 15)
	testutil.WriteFileSized(t, filepath.Join(dirB, "f3.bin"), 8)

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


func TestDirectoriesExcludesHardLinkedFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard link test requires unix")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "node_modules")
	storeDir := filepath.Join(root, "store")
	testutil.MkdirAll(t, dir)
	testutil.MkdirAll(t, storeDir)

	// Create a regular (unique) file – should be counted.
	testutil.WriteFileSized(t, filepath.Join(dir, "unique.bin"), 100)

	// Create a file in the "store" and hard-link it into the directory.
	// The hard-linked copy has Nlink > 1 and should NOT be counted.
	storePath := filepath.Join(storeDir, "shared.bin")
	testutil.WriteFileSized(t, storePath, 200)
	linkPath := filepath.Join(dir, "shared.bin")
	if err := os.Link(storePath, linkPath); err != nil {
		t.Fatalf("hard link: %v", err)
	}

	sizes, errs := Directories([]string{dir}, 1)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	// Only the unique file should be counted.
	if sizes[dir] != 100 {
		t.Fatalf("expected size 100 (unique only), got %d", sizes[dir])
	}
}

func TestDirectoriesCountsAllWhenNoHardLinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plain")
	testutil.MkdirAll(t, dir)

	testutil.WriteFileSized(t, filepath.Join(dir, "a.bin"), 30)
	testutil.WriteFileSized(t, filepath.Join(dir, "b.bin"), 70)

	sizes, errs := Directories([]string{dir}, 1)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[dir] != 100 {
		t.Fatalf("expected size 100, got %d", sizes[dir])
	}
}

