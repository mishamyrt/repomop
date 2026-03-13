package scanner

import (
	"context"

	"repomop/internal/size"
)

// ScanAndMeasure scans for artifacts, measures their sizes, and returns them
// sorted by size descending. Non-fatal warnings from scanning and size
// calculation are collected and returned alongside the results.
func ScanAndMeasure(ctx context.Context, opts ScanOptions) ([]Artifact, []error, error) {
	artifacts, scanWarnings, err := Scan(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	paths := make([]string, 0, len(artifacts))
	for _, a := range artifacts {
		paths = append(paths, a.Path)
	}

	sizes, sizeWarnings := size.Directories(ctx, paths, size.RecommendedWorkerCount())
	for i := range artifacts {
		artifacts[i].SizeBytes = sizes[artifacts[i].Path]
	}

	SortBySizeDesc(artifacts)

	warnings := make([]error, 0, len(scanWarnings)+len(sizeWarnings))
	warnings = append(warnings, scanWarnings...)
	warnings = append(warnings, sizeWarnings...)
	return artifacts, warnings, nil
}
