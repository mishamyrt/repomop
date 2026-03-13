package delete

import (
	"context"
	"os"

	"repomop/internal/scanner"
)

// Error captures an artifact that could not be removed.
type Error struct {
	Artifact scanner.Artifact
	Err      error
}

// Result reports deletion outcomes.
type Result struct {
	Deleted    []scanner.Artifact
	Errors     []Error
	FreedBytes int64
}

// Artifacts removes selected artifact directories using os.RemoveAll.
// It stops processing further artifacts if ctx is cancelled.
func Artifacts(ctx context.Context, artifacts []scanner.Artifact) Result {
	result := Result{
		Deleted: make([]scanner.Artifact, 0, len(artifacts)),
		Errors:  nil,
	}

	for _, artifact := range artifacts {
		if ctx.Err() != nil {
			break
		}
		if err := os.RemoveAll(artifact.Path); err != nil {
			result.Errors = append(result.Errors, Error{Artifact: artifact, Err: err})
			continue
		}
		result.Deleted = append(result.Deleted, artifact)
		result.FreedBytes += artifact.SizeBytes
	}

	return result
}
