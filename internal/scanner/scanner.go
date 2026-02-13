package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type walkItem struct {
	path  string
	depth int
}

// Scan recursively traverses the root directory and finds known build artifacts.
func Scan(opts ScanOptions) ([]Artifact, error) {
	if opts.RootPath == "" {
		return nil, fmt.Errorf("root path is required")
	}

	root, err := filepath.Abs(opts.RootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve root path: %w", err)
	}
	root = filepath.Clean(root)

	rootInfo, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat root path: %w", err)
	}
	if !rootInfo.IsDir() {
		return nil, fmt.Errorf("root path is not a directory: %s", root)
	}

	stack := []walkItem{{path: root, depth: 0}}
	artifacts := make([]Artifact, 0, 64)

	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		entries, err := os.ReadDir(item.path)
		if err != nil {
			return nil, fmt.Errorf("read directory %s: %w", item.path, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				if entry.Type()&fs.ModeSymlink != 0 {
					continue
				}
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

			if artifact, ok := detectArtifact(childPath, root); ok {
				artifacts = append(artifacts, artifact)
				continue
			}

			stack = append(stack, walkItem{path: childPath, depth: childDepth})
		}
	}

	return artifacts, nil
}
