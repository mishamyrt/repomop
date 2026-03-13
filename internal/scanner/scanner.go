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

	for len(stack) > 0 {
		if ctx.Err() != nil {
			break
		}

		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

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

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if entry.Type()&fs.ModeSymlink != 0 {
				continue
			}

			childPath := filepath.Join(item.path, entry.Name())
			childDepth := item.depth + 1
			if opts.MaxDepth >= 0 && childDepth > opts.MaxDepth {
				continue
			}

			if artifact, ok := detectArtifact(scanCtx, childPath); ok {
				artifacts = append(artifacts, artifact)
				continue
			}

			stack = append(stack, walkItem{path: childPath, depth: childDepth})
		}
	}

	return artifacts, warnings, nil
}
