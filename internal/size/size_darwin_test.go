//go:build darwin

package size

import (
	"context"
	"path/filepath"
	"testing"

	"repomop/internal/testutil"

	"golang.org/x/sys/unix"
)

const largeFileSize = 1024 * 1024 // 1 MiB, well above APFS inline threshold

func TestDirectoriesExcludesCOWClones(t *testing.T) {
	root := t.TempDir()
	storeDir := filepath.Join(root, "store")
	dir := filepath.Join(root, "node_modules")
	testutil.MkdirAll(t, storeDir)
	testutil.MkdirAll(t, dir)

	testutil.WriteFileSized(t, filepath.Join(dir, "unique.bin"), largeFileSize)

	storePath := filepath.Join(storeDir, "shared.bin")
	testutil.WriteFileSized(t, storePath, largeFileSize)

	clonePath := filepath.Join(dir, "clone.bin")
	if err := unix.Clonefile(storePath, clonePath, 0); err != nil {
		t.Skipf("clonefile not supported: %v", err)
	}

	sizes, errs := Directories(context.Background(), []string{dir}, 1, Options{})
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %d", len(errs))
	}

	if sizes[dir] != int64(largeFileSize) {
		t.Fatalf("expected size %d (clone excluded), got %d", largeFileSize, sizes[dir])
	}
}

func TestPrivateDataSizeReturnsZeroForClone(t *testing.T) {
	root := t.TempDir()
	original := filepath.Join(root, "original.bin")
	clone := filepath.Join(root, "clone.bin")

	testutil.WriteFileSized(t, original, largeFileSize)

	if err := unix.Clonefile(original, clone, 0); err != nil {
		t.Skipf("clonefile not supported: %v", err)
	}

	ps, err := privateDataSize(clone)
	if err != nil {
		t.Fatalf("privateDataSize: %v", err)
	}
	if ps != 0 {
		t.Fatalf("expected private size 0 for clone, got %d", ps)
	}
}

func TestPrivateDataSizeReturnsFullForRegularFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "regular.bin")
	testutil.WriteFileSized(t, path, largeFileSize)

	ps, err := privateDataSize(path)
	if err != nil {
		t.Fatalf("privateDataSize: %v", err)
	}
	if ps != int64(largeFileSize) {
		t.Fatalf("expected private size %d for regular file, got %d", largeFileSize, ps)
	}
}
