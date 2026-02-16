package scanner

import (
	"path/filepath"
	"runtime"
	"testing"

	"repomop/internal/testutil"
)

func TestScanNodeModulesRequiresPackageJSON(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "js-project")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "node_modules"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	nodeArtifacts := artifactsByKind(artifacts, ArtifactNodeModule)
	if len(nodeArtifacts) != 1 {
		t.Fatalf("expected 1 node_modules artifact, got %d", len(nodeArtifacts))
	}
	if nodeArtifacts[0].Path != filepath.Join(project, "node_modules") {
		t.Fatalf("unexpected node_modules path: %s", nodeArtifacts[0].Path)
	}
	if nodeArtifacts[0].ProjectRoot != project {
		t.Fatalf("unexpected project root: %s", nodeArtifacts[0].ProjectRoot)
	}
}

func TestScanTargetRequiresCargoToml(t *testing.T) {
	root := t.TempDir()

	rustProject := filepath.Join(root, "rust")
	testutil.MkdirAll(t, filepath.Join(rustProject, "target"))
	testutil.WriteFile(t, filepath.Join(rustProject, "Cargo.toml"), "[package]\nname='x'\n")

	other := filepath.Join(root, "other")
	testutil.MkdirAll(t, filepath.Join(other, "target"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	rustArtifacts := artifactsByKind(artifacts, ArtifactRustTarget)
	if len(rustArtifacts) != 1 {
		t.Fatalf("expected 1 rust target artifact, got %d", len(rustArtifacts))
	}
	if rustArtifacts[0].Path != filepath.Join(rustProject, "target") {
		t.Fatalf("unexpected rust target path: %s", rustArtifacts[0].Path)
	}
	if rustArtifacts[0].ProjectRoot != rustProject {
		t.Fatalf("unexpected rust project root: %s", rustArtifacts[0].ProjectRoot)
	}
}

func TestScanSwiftBuildRequiresPackageSwift(t *testing.T) {
	root := t.TempDir()

	swiftProject := filepath.Join(root, "swift")
	testutil.MkdirAll(t, filepath.Join(swiftProject, ".build"))
	testutil.WriteFile(t, filepath.Join(swiftProject, "Package.swift"), "// swift-tools-version: 5.9")

	other := filepath.Join(root, "other")
	testutil.MkdirAll(t, filepath.Join(other, ".build"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	swiftArtifacts := artifactsByKind(artifacts, ArtifactSwiftBuild)
	if len(swiftArtifacts) != 1 {
		t.Fatalf("expected 1 swift build artifact, got %d", len(swiftArtifacts))
	}
	if swiftArtifacts[0].Path != filepath.Join(swiftProject, ".build") {
		t.Fatalf("unexpected swift build path: %s", swiftArtifacts[0].Path)
	}
	if swiftArtifacts[0].ProjectRoot != swiftProject {
		t.Fatalf("unexpected swift project root: %s", swiftArtifacts[0].ProjectRoot)
	}
}

func TestScanGradleArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "gradle")
	testutil.MkdirAll(t, filepath.Join(project, ".gradle"))
	testutil.MkdirAll(t, filepath.Join(project, "build"))
	testutil.MkdirAll(t, filepath.Join(project, "out"))
	testutil.WriteFile(t, filepath.Join(project, "settings.gradle"), "rootProject.name = 'x'")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, ".gradle"),
		filepath.Join(project, "build"),
		filepath.Join(project, "out"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected gradle artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactJavaGradle {
			t.Fatalf("expected java-gradle kind, got %s", artifact.Kind)
		}
		if artifact.ProjectRoot != project {
			t.Fatalf("unexpected gradle project root %s", artifact.ProjectRoot)
		}
	}
}

func TestScanTargetSupportsMavenAndRust(t *testing.T) {
	root := t.TempDir()

	rustProject := filepath.Join(root, "rust")
	testutil.MkdirAll(t, filepath.Join(rustProject, "target"))
	testutil.WriteFile(t, filepath.Join(rustProject, "Cargo.toml"), "[package]\nname='x'\n")

	mavenProject := filepath.Join(root, "maven")
	testutil.MkdirAll(t, filepath.Join(mavenProject, "target"))
	testutil.WriteFile(t, filepath.Join(mavenProject, "pom.xml"), "<project/>")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "target"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	rustTarget, ok := artifactAtPath(artifacts, filepath.Join(rustProject, "target"))
	if !ok {
		t.Fatalf("expected rust target artifact")
	}
	if rustTarget.Kind != ArtifactRustTarget {
		t.Fatalf("expected rust-target kind, got %s", rustTarget.Kind)
	}

	mavenTarget, ok := artifactAtPath(artifacts, filepath.Join(mavenProject, "target"))
	if !ok {
		t.Fatalf("expected maven target artifact")
	}
	if mavenTarget.Kind != ArtifactJavaMaven {
		t.Fatalf("expected java-maven kind, got %s", mavenTarget.Kind)
	}

	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, "target")); ok {
		t.Fatalf("unexpected orphan target artifact")
	}
}

func TestScanCMakeArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "cmake")
	testutil.MkdirAll(t, filepath.Join(project, "build"))
	testutil.MkdirAll(t, filepath.Join(project, "cmake-build-debug"))
	testutil.MkdirAll(t, filepath.Join(project, "CMakeFiles"))
	testutil.WriteFile(t, filepath.Join(project, "CMakeLists.txt"), "cmake_minimum_required(VERSION 3.20)")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, "build"),
		filepath.Join(project, "cmake-build-debug"),
		filepath.Join(project, "CMakeFiles"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected cmake artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactCMake {
			t.Fatalf("expected cmake kind, got %s", artifact.Kind)
		}
	}
}

func TestScanFlutterArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "flutter")
	testutil.MkdirAll(t, filepath.Join(project, ".dart_tool"))
	testutil.MkdirAll(t, filepath.Join(project, "build"))
	testutil.WriteFile(t, filepath.Join(project, "pubspec.yaml"), "name: x")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, ".dart_tool"),
		filepath.Join(project, "build"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected flutter artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactFlutter {
			t.Fatalf("expected dart-flutter kind, got %s", artifact.Kind)
		}
	}
}

func TestScanRubyArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "ruby")
	testutil.MkdirAll(t, filepath.Join(project, ".bundle"))
	testutil.MkdirAll(t, filepath.Join(project, "vendor", "bundle"))
	testutil.WriteFile(t, filepath.Join(project, "Gemfile"), "source 'https://rubygems.org'")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, ".bundle"),
		filepath.Join(project, "vendor", "bundle"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected ruby artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactRuby {
			t.Fatalf("expected ruby kind, got %s", artifact.Kind)
		}
	}
}

func TestScanPHPVendorRequiresComposerJSON(t *testing.T) {
	root := t.TempDir()

	phpProject := filepath.Join(root, "php")
	testutil.MkdirAll(t, filepath.Join(phpProject, "vendor"))
	testutil.WriteFile(t, filepath.Join(phpProject, "composer.json"), "{}")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "vendor"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	phpVendor, ok := artifactAtPath(artifacts, filepath.Join(phpProject, "vendor"))
	if !ok {
		t.Fatalf("expected php vendor artifact")
	}
	if phpVendor.Kind != ArtifactPHP {
		t.Fatalf("expected php kind, got %s", phpVendor.Kind)
	}
	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, "vendor")); ok {
		t.Fatalf("unexpected orphan vendor artifact")
	}
}

func TestScanZigArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "zig-project")
	testutil.MkdirAll(t, filepath.Join(project, "zig-out"))
	testutil.MkdirAll(t, filepath.Join(project, ".zig-cache"))
	testutil.WriteFile(t, filepath.Join(project, "build.zig"), "const std = @import(\"std\");")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "zig-out"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, "zig-out"),
		filepath.Join(project, ".zig-cache"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected zig artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactZig {
			t.Fatalf("expected zig kind, got %s", artifact.Kind)
		}
		if artifact.ProjectRoot != project {
			t.Fatalf("unexpected zig project root: %s", artifact.ProjectRoot)
		}
	}

	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, "zig-out")); ok {
		t.Fatalf("unexpected orphan zig-out artifact")
	}
}

func TestScanBuildUsesNearestProjectMarker(t *testing.T) {
	root := t.TempDir()

	testutil.WriteFile(t, filepath.Join(root, "settings.gradle"), "rootProject.name = 'mono'")

	flutterProject := filepath.Join(root, "apps", "flutter-app")
	testutil.MkdirAll(t, filepath.Join(flutterProject, "build"))
	testutil.WriteFile(t, filepath.Join(flutterProject, "pubspec.yaml"), "name: app")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})
	artifact, ok := artifactAtPath(artifacts, filepath.Join(flutterProject, "build"))
	if !ok {
		t.Fatalf("expected build artifact in flutter project")
	}
	if artifact.Kind != ArtifactFlutter {
		t.Fatalf("expected flutter artifact by nearest marker, got %s", artifact.Kind)
	}
	if artifact.ProjectRoot != flutterProject {
		t.Fatalf("unexpected project root: %s", artifact.ProjectRoot)
	}
}

func TestScanVirtualEnvByPyvenvCfg(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "python")
	venv := filepath.Join(project, "custom_env")
	testutil.MkdirAll(t, venv)
	testutil.WriteFile(t, filepath.Join(venv, "pyvenv.cfg"), "home = /usr/bin")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	venvArtifacts := artifactsByKind(artifacts, ArtifactPythonVenv)
	if len(venvArtifacts) != 1 {
		t.Fatalf("expected 1 python venv artifact, got %d", len(venvArtifacts))
	}
	if venvArtifacts[0].Path != venv {
		t.Fatalf("unexpected venv path: %s", venvArtifacts[0].Path)
	}
	if venvArtifacts[0].ProjectRoot != project {
		t.Fatalf("unexpected venv project root: %s", venvArtifacts[0].ProjectRoot)
	}
}

func TestScanVirtualEnvByBinMarkers(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "python")
	venv := filepath.Join(project, "sandbox")
	testutil.MkdirAll(t, filepath.Join(venv, "bin"))
	testutil.WriteFile(t, filepath.Join(venv, "bin", "activate"), "source")
	testutil.WriteFile(t, filepath.Join(venv, "bin", "python"), "")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	venvArtifacts := artifactsByKind(artifacts, ArtifactPythonVenv)
	if len(venvArtifacts) != 1 {
		t.Fatalf("expected 1 python venv artifact, got %d", len(venvArtifacts))
	}
	if venvArtifacts[0].Path != venv {
		t.Fatalf("unexpected venv path: %s", venvArtifacts[0].Path)
	}
}

func TestScanSkipsSymlinkDirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink requires privileges on windows")
	}

	root := t.TempDir()
	project := filepath.Join(root, "project")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")

	testutil.Symlink(t, project, filepath.Join(root, "project-link"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	nodeArtifacts := artifactsByKind(artifacts, ArtifactNodeModule)
	if len(nodeArtifacts) != 1 {
		t.Fatalf("expected 1 node_modules artifact, got %d", len(nodeArtifacts))
	}
}

func TestScanRespectsMaxDepth(t *testing.T) {
	root := t.TempDir()

	nested := filepath.Join(root, "a", "b", "c")
	testutil.MkdirAll(t, filepath.Join(nested, "node_modules"))
	testutil.WriteFile(t, filepath.Join(nested, "package.json"), "{}")

	artifactsDepth2 := mustScan(t, ScanOptions{RootPath: root, MaxDepth: 2})
	if len(artifactsByKind(artifactsDepth2, ArtifactNodeModule)) != 0 {
		t.Fatalf("expected no artifacts with max depth 2")
	}

	artifactsDepth4 := mustScan(t, ScanOptions{RootPath: root, MaxDepth: 4})
	if len(artifactsByKind(artifactsDepth4, ArtifactNodeModule)) != 1 {
		t.Fatalf("expected artifact with max depth 4")
	}
}

func mustScan(t *testing.T, opts ScanOptions) []Artifact {
	t.Helper()
	artifacts, _, err := Scan(opts)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	return artifacts
}

func artifactsByKind(artifacts []Artifact, kind ArtifactKind) []Artifact {
	filtered := make([]Artifact, 0)
	for _, artifact := range artifacts {
		if artifact.Kind == kind {
			filtered = append(filtered, artifact)
		}
	}
	return filtered
}

func artifactAtPath(artifacts []Artifact, path string) (Artifact, bool) {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact, true
		}
	}
	return Artifact{}, false
}

