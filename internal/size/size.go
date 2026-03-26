package size

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
)

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
func Directories(ctx context.Context, paths []string, workers int) (map[string]int64, []error) {
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
				size, warnings := directorySize(path)
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

func directorySize(path string) (int64, []error) {
	var total int64
	var warnings []error

	_ = filepath.WalkDir(path, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, err)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			warnings = append(warnings, err)
			return nil
		}
		total += reclaimableFileSize(filePath, info)
		return nil
	})

	return total, warnings
}
