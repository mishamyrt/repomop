package scanner

import "sort"

// ArtifactKind classifies supported removable development artifacts.
type ArtifactKind string

// Artifact kind values used by the scanner and UI.
const (
	ArtifactPythonVenv ArtifactKind = "python-venv"
	ArtifactNodeModule ArtifactKind = "node-modules"
	ArtifactRustTarget ArtifactKind = "rust-target"
	ArtifactSwiftBuild ArtifactKind = "swift-build"
	ArtifactJavaGradle ArtifactKind = "java-gradle"
	ArtifactJavaMaven  ArtifactKind = "java-maven"
	ArtifactCMake      ArtifactKind = "cmake"
	ArtifactFlutter    ArtifactKind = "dart-flutter"
	ArtifactRuby       ArtifactKind = "ruby"
	ArtifactPHP        ArtifactKind = "php"
	ArtifactZig        ArtifactKind = "zig"
)

// Artifact describes a single removable artifact directory.
type Artifact struct {
	Kind        ArtifactKind
	Path        string
	ProjectRoot string
	SizeBytes   int64
}

// SortBySizeDesc sorts artifacts by SizeBytes descending, then by Path ascending.
func SortBySizeDesc(artifacts []Artifact) {
	sort.SliceStable(artifacts, func(i, j int) bool {
		if artifacts[i].SizeBytes == artifacts[j].SizeBytes {
			return artifacts[i].Path < artifacts[j].Path
		}
		return artifacts[i].SizeBytes > artifacts[j].SizeBytes
	})
}

// ScanOptions configures repository scanning.
type ScanOptions struct {
	RootPath string
	MaxDepth int
}
