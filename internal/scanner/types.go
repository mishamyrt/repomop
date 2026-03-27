package scanner

import (
	"cmp"
	"slices"
)

// ArtifactKind classifies supported removable development artifacts.
type ArtifactKind uint8

// Artifact kind values used by the scanner and UI.
const (
	ArtifactPythonVenv ArtifactKind = iota
	ArtifactNodeModule
	ArtifactRustTarget
	ArtifactSwiftBuild
	ArtifactElixir
	ArtifactHaskell
	ArtifactTerraform
	ArtifactJavaGradle
	ArtifactJavaMaven
	ArtifactCMake
	ArtifactFlutter
	ArtifactRuby
	ArtifactPHP
	ArtifactZig
	ArtifactPlatformIO
)

var artifactKindNames = [...]string{
	ArtifactPythonVenv: "python-venv",
	ArtifactNodeModule: "node-modules",
	ArtifactRustTarget: "rust-target",
	ArtifactSwiftBuild: "swift-build",
	ArtifactElixir:     "elixir",
	ArtifactHaskell:    "haskell",
	ArtifactTerraform:  "terraform",
	ArtifactJavaGradle: "java-gradle",
	ArtifactJavaMaven:  "java-maven",
	ArtifactCMake:      "cmake",
	ArtifactFlutter:    "dart-flutter",
	ArtifactRuby:       "ruby",
	ArtifactPHP:        "php",
	ArtifactZig:        "zig",
	ArtifactPlatformIO: "platformio",
}

// String returns the display name of the artifact kind.
func (k ArtifactKind) String() string {
	if int(k) < len(artifactKindNames) {
		return artifactKindNames[k]
	}
	return "unknown"
}

// Artifact describes a single removable artifact directory.
type Artifact struct {
	Kind        ArtifactKind
	Path        string
	ProjectRoot string
	SizeBytes   int64
}

// SortBySizeDesc sorts artifacts by SizeBytes descending, then by Path ascending.
func SortBySizeDesc(artifacts []Artifact) {
	slices.SortStableFunc(artifacts, func(a, b Artifact) int {
		if a.SizeBytes != b.SizeBytes {
			return cmp.Compare(b.SizeBytes, a.SizeBytes)
		}
		return cmp.Compare(a.Path, b.Path)
	})
}

// ScanOptions configures repository scanning.
type ScanOptions struct {
	RootPath     string
	MaxDepth     int
	IncludeLinks bool
}
