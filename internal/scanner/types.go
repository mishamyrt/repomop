package scanner

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
)

// Artifact describes a single removable artifact directory.
type Artifact struct {
	Kind        ArtifactKind
	Path        string
	ProjectRoot string
	SizeBytes   int64
}

// ScanOptions configures repository scanning.
type ScanOptions struct {
	RootPath       string
	MaxDepth       int
	FollowSymlinks bool
}
