package size

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"sync"
)

type jobResult struct {
	path string
	size int64
	err  error
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
func Directories(paths []string, workers int) (map[string]int64, []error) {
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
				size, err := directorySize(path)
				results <- jobResult{path: path, size: size, err: err}
			}
		}()
	}

	go func() {
		for _, path := range paths {
			jobs <- path
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	errs := make([]error, 0)
	for result := range results {
		sizes[result.path] = result.size
		if result.err != nil {
			errs = append(errs, result.err)
		}
	}

	return sizes, errs
}

func directorySize(path string) (int64, error) {
	var total int64

	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
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
			return err
		}
		total += info.Size()
		return nil
	})

	return total, err
}
