package size

import (
	"context"
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

	sizes, errs := Directories(context.Background(), []string{dirA, dirB}, 2, Options{})
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

	testutil.WriteFileSized(t, filepath.Join(dir, "unique.bin"), 100)

	storePath := filepath.Join(storeDir, "shared.bin")
	testutil.WriteFileSized(t, storePath, 200)
	linkPath := filepath.Join(dir, "shared.bin")
	if err := os.Link(storePath, linkPath); err != nil {
		t.Fatalf("hard link: %v", err)
	}

	sizes, errs := Directories(context.Background(), []string{dir}, 1, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

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

	sizes, errs := Directories(context.Background(), []string{dir}, 1, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[dir] != 100 {
		t.Fatalf("expected size 100, got %d", sizes[dir])
	}
}

func TestDirectoriesEmptyPaths(t *testing.T) {
	sizes, errs := Directories(context.Background(), []string{}, 1, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if len(sizes) != 0 {
		t.Fatalf("expected empty sizes map, got %d entries", len(sizes))
	}
}

func TestDirectoriesWorkersClampedToOne(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "d")
	testutil.MkdirAll(t, dir)
	testutil.WriteFileSized(t, filepath.Join(dir, "f.bin"), 42)

	sizes, errs := Directories(context.Background(), []string{dir}, 0, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if sizes[dir] != 42 {
		t.Fatalf("expected 42, got %d", sizes[dir])
	}
}

func TestDirectoriesSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires unix")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "project")
	testutil.MkdirAll(t, dir)

	external := filepath.Join(root, "external")
	testutil.MkdirAll(t, external)
	testutil.WriteFileSized(t, filepath.Join(external, "big.bin"), 9999)

	testutil.Symlink(t, external, filepath.Join(dir, "link"))

	testutil.WriteFileSized(t, filepath.Join(dir, "real.bin"), 50)

	sizes, _ := Directories(context.Background(), []string{dir}, 1, Options{})
	if sizes[dir] != 50 {
		t.Fatalf("expected 50 (symlink excluded), got %d", sizes[dir])
	}
}

func TestDirectoriesNonexistentPath(t *testing.T) {
	sizes, _ := Directories(context.Background(), []string{"/nonexistent/path/xyz"}, 1, Options{})
	if sizes["/nonexistent/path/xyz"] != 0 {
		t.Fatalf("expected 0 for nonexistent path, got %d", sizes["/nonexistent/path/xyz"])
	}
}

func TestRecommendedWorkerCount(t *testing.T) {
	w := RecommendedWorkerCount()
	if w < 1 {
		t.Fatalf("expected at least 1 worker, got %d", w)
	}
	if w > 8 {
		t.Fatalf("expected at most 8 workers, got %d", w)
	}
}

func TestDirectoriesEmptyDirectory(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "empty")
	testutil.MkdirAll(t, dir)

	sizes, errs := Directories(context.Background(), []string{dir}, 1, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	if sizes[dir] != 0 {
		t.Fatalf("expected 0 for empty dir, got %d", sizes[dir])
	}
}

func TestDirectorySizeWithUnreadableSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not supported on windows")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "project")
	testutil.MkdirAll(t, dir)
	testutil.WriteFileSized(t, filepath.Join(dir, "a.bin"), 50)

	unreadable := filepath.Join(dir, "noperm")
	testutil.MkdirAll(t, unreadable)
	testutil.WriteFileSized(t, filepath.Join(unreadable, "b.bin"), 100)

	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o755) })

	sizes, warnings := Directories(context.Background(), []string{dir}, 1, Options{})
	if len(warnings) == 0 {
		t.Fatal("expected warnings for unreadable subdir")
	}
	if sizes[dir] != 50 {
		t.Fatalf("expected 50 (only readable file), got %d", sizes[dir])
	}
}

func TestDirectoriesMultipleWorkers(t *testing.T) {
	root := t.TempDir()
	dirs := make([]string, 5)
	for i := range dirs {
		d := filepath.Join(root, string(rune('a'+i)))
		testutil.MkdirAll(t, d)
		testutil.WriteFileSized(t, filepath.Join(d, "f.bin"), (i+1)*10)
		dirs[i] = d
	}

	sizes, errs := Directories(context.Background(), dirs, 4, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}
	for i, d := range dirs {
		expected := int64((i + 1) * 10)
		if sizes[d] != expected {
			t.Fatalf("dir %d: expected %d, got %d", i, expected, sizes[d])
		}
	}
}

func TestDirectoriesIncludesHardLinkedFilesWhenRequested(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hard link test requires unix")
	}

	root := t.TempDir()
	dir := filepath.Join(root, "node_modules")
	storeDir := filepath.Join(root, "store")
	testutil.MkdirAll(t, dir)
	testutil.MkdirAll(t, storeDir)

	testutil.WriteFileSized(t, filepath.Join(dir, "unique.bin"), 100)

	storePath := filepath.Join(storeDir, "shared.bin")
	testutil.WriteFileSized(t, storePath, 200)
	linkPath := filepath.Join(dir, "shared.bin")
	if err := os.Link(storePath, linkPath); err != nil {
		t.Fatalf("hard link: %v", err)
	}

	sizes, errs := Directories(context.Background(), []string{dir}, 1, Options{IncludeLinks: true})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[dir] != 300 {
		t.Fatalf("expected size 300 (including hard link), got %d", sizes[dir])
	}
}

func TestDirectoriesIncludesSymlinkedRootWhenRequested(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink test requires unix")
	}

	root := t.TempDir()
	target := filepath.Join(root, "target")
	link := filepath.Join(root, "node_modules")
	testutil.MkdirAll(t, target)
	testutil.WriteFileSized(t, filepath.Join(target, "dep.bin"), 90)
	testutil.Symlink(t, target, link)

	sizes, errs := Directories(context.Background(), []string{link}, 1, Options{IncludeLinks: true})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[link] != 90 {
		t.Fatalf("expected size 90 (following symlink), got %d", sizes[link])
	}
}
