package delete

import (
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
func Artifacts(artifacts []scanner.Artifact) Result {
	result := Result{
		Deleted: make([]scanner.Artifact, 0, len(artifacts)),
		Errors:  make([]Error, 0),
	}

	for _, artifact := range artifacts {
		if err := os.RemoveAll(artifact.Path); err != nil {
			result.Errors = append(result.Errors, Error{Artifact: artifact, Err: err})
			continue
		}
		result.Deleted = append(result.Deleted, artifact)
		result.FreedBytes += artifact.SizeBytes
	}

	return result
}
