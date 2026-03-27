package size

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// Options configures directory size calculation.
type Options struct {
	IncludeLinks bool
}

type jobResult struct {
	path     string
	size     int64
	warnings []error
}

// RecommendedWorkerCount returns a bounded worker count for responsive scans.
func RecommendedWorkerCount() int {
	workers := runtime.NumCPU()
	if workers < 1 {
		workers = 1
	}
	if workers > 8 {
		workers = 8
	}
	return workers
}

// Directories calculates recursive sizes for each directory path.
// It stops dispatching new jobs if ctx is cancelled, but waits for
// already-running jobs to finish before returning.
func Directories(ctx context.Context, paths []string, workers int, opts Options) (map[string]int64, []error) {
	if workers < 1 {
		workers = 1
	}

	sizes := make(map[string]int64, len(paths))
	if len(paths) == 0 {
		return sizes, nil
	}

	jobs := make(chan string)
	results := make(chan jobResult)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				size, warnings := directorySize(path, opts)
				results <- jobResult{path: path, size: size, warnings: warnings}
			}
		}()
	}

	go func() {
		defer close(results)
		defer wg.Wait()
		defer close(jobs)
		for _, path := range paths {
			select {
			case <-ctx.Done():
				return
			case jobs <- path:
			}
		}
	}()

	var warnings []error
	for result := range results {
		sizes[result.path] = result.size
		warnings = append(warnings, result.warnings...)
	}

	return sizes, warnings
}

func directorySize(path string, opts Options) (int64, []error) {
	walker := sizeWalker{
		opts:        opts,
		visitedDirs: make(map[string]struct{}),
	}
	walker.walk(path)
	return walker.total, walker.warnings
}

type sizeWalker struct {
	opts        Options
	total       int64
	warnings    []error
	visitedDirs map[string]struct{}
}

func (w *sizeWalker) walk(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		w.warnings = append(w.warnings, err)
		return
	}

	mode := info.Mode()
	if mode&os.ModeSymlink != 0 {
		if !w.opts.IncludeLinks {
			return
		}

		targetInfo, err := os.Stat(path)
		if err != nil {
			w.warnings = append(w.warnings, err)
			return
		}
		if targetInfo.IsDir() {
			w.walkDirectory(path)
			return
		}
		if targetInfo.Mode().IsRegular() {
			w.total += reclaimableFileSize(path, targetInfo, true)
		}
		return
	}

	if info.IsDir() {
		w.walkDirectory(path)
		return
	}
	if !mode.IsRegular() {
		return
	}

	w.total += reclaimableFileSize(path, info, w.opts.IncludeLinks)
}

func (w *sizeWalker) walkDirectory(path string) {
	if w.opts.IncludeLinks {
		realPath, err := filepath.EvalSymlinks(path)
		if err != nil {
			w.warnings = append(w.warnings, err)
			return
		}
		realPath = filepath.Clean(realPath)
		if _, ok := w.visitedDirs[realPath]; ok {
			return
		}
		w.visitedDirs[realPath] = struct{}{}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		w.warnings = append(w.warnings, err)
		return
	}
	for _, entry := range entries {
		w.walk(filepath.Join(path, entry.Name()))
	}
}
