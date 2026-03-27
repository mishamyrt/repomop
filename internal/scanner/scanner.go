package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type walkItem struct {
	path  string
	depth int
}

// initialArtifactCapacity is a pre-allocation hint for typical project counts.
const initialArtifactCapacity = 64

// Scan recursively traverses the root directory and finds known build artifacts.
// Non-fatal directory read errors are collected as warnings instead of aborting.
// The scan is cancelled early if ctx is done.
func Scan(ctx context.Context, opts ScanOptions) ([]Artifact, []error, error) {
	if opts.RootPath == "" {
		return nil, nil, fmt.Errorf("root path is required")
	}

	root := filepath.Clean(opts.RootPath)

	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, nil, fmt.Errorf("stat root path: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	scanCtx := newScanContext(root)
	stack := []walkItem{{path: root, depth: 0}}
	artifacts := make([]Artifact, 0, initialArtifactCapacity)
	var warnings []error
	visitedDirs := make(map[string]struct{})

	for len(stack) > 0 {
		if ctx.Err() != nil {
			break
		}

		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if opts.IncludeLinks {
			realPath, err := filepath.EvalSymlinks(item.path)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("resolve directory %s: %w", item.path, err))
				continue
			}
			realPath = filepath.Clean(realPath)
			if _, ok := visitedDirs[realPath]; ok {
				continue
			}
			visitedDirs[realPath] = struct{}{}
		}

		f, err := os.Open(item.path)
		if err != nil {
			warnings = append(warnings, fmt.Errorf("read directory %s: %w", item.path, err))
			continue
		}
		entries, err := f.ReadDir(-1)
		f.Close()
		if err != nil {
			warnings = append(warnings, fmt.Errorf("read directory %s: %w", item.path, err))
			continue
		}

		symlinkChildren := make([]walkItem, 0)
		regularChildren := make([]walkItem, 0, len(entries))
		for _, entry := range entries {
			childPath := filepath.Join(item.path, entry.Name())
			isDir, isSymlinkDir, err := directoryEntryStatus(childPath, entry, opts.IncludeLinks)
			if err != nil {
				warnings = append(warnings, fmt.Errorf("resolve directory %s: %w", childPath, err))
				continue
			}
			if !isDir {
				continue
			}

			childDepth := item.depth + 1
			if opts.MaxDepth >= 0 && childDepth > opts.MaxDepth {
				continue
			}

			if artifact, ok := detectArtifact(scanCtx, childPath); ok {
				artifacts = append(artifacts, artifact)
				continue
			}

			child := walkItem{path: childPath, depth: childDepth}
			if isSymlinkDir {
				symlinkChildren = append(symlinkChildren, child)
				continue
			}
			regularChildren = append(regularChildren, child)
		}

		stack = append(stack, symlinkChildren...)
		stack = append(stack, regularChildren...)
	}

	return artifacts, warnings, nil
}

func directoryEntryStatus(path string, entry fs.DirEntry, includeLinks bool) (bool, bool, error) {
	if entry.IsDir() {
		return true, false, nil
	}
	if entry.Type()&fs.ModeSymlink == 0 || !includeLinks {
		return false, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, true, err
	}
	return info.IsDir(), true, nil
}
