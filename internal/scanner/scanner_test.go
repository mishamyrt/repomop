package scanner

import (
	"context"
	"os"
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

func TestScanElixirArtifacts(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "elixir")
	testutil.MkdirAll(t, filepath.Join(project, "_build"))
	testutil.MkdirAll(t, filepath.Join(project, "deps"))
	testutil.WriteFile(t, filepath.Join(project, "mix.exs"), "defmodule Sample.MixProject do end")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "_build"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	for _, artifactPath := range []string{
		filepath.Join(project, "_build"),
		filepath.Join(project, "deps"),
	} {
		artifact, ok := artifactAtPath(artifacts, artifactPath)
		if !ok {
			t.Fatalf("expected elixir artifact %s", artifactPath)
		}
		if artifact.Kind != ArtifactElixir {
			t.Fatalf("expected elixir kind, got %s", artifact.Kind)
		}
		if artifact.ProjectRoot != project {
			t.Fatalf("unexpected elixir project root: %s", artifact.ProjectRoot)
		}
	}

	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, "_build")); ok {
		t.Fatalf("unexpected orphan elixir _build artifact")
	}
}

func TestScanHaskellArtifacts(t *testing.T) {
	root := t.TempDir()

	stackProject := filepath.Join(root, "stack-app")
	testutil.MkdirAll(t, filepath.Join(stackProject, ".stack-work"))
	testutil.WriteFile(t, filepath.Join(stackProject, "stack.yaml"), "resolver: lts-22.0")
	testutil.WriteFile(t, filepath.Join(stackProject, "stack-app.cabal"), "name: stack-app")

	cabalProject := filepath.Join(root, "cabal-app")
	testutil.MkdirAll(t, filepath.Join(cabalProject, "dist-newstyle"))
	testutil.WriteFile(t, filepath.Join(cabalProject, "cabal-app.cabal"), "name: cabal-app")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, "dist-newstyle"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	stackArtifact, ok := artifactAtPath(artifacts, filepath.Join(stackProject, ".stack-work"))
	if !ok {
		t.Fatalf("expected stack work artifact")
	}
	if stackArtifact.Kind != ArtifactHaskell {
		t.Fatalf("expected haskell kind, got %s", stackArtifact.Kind)
	}
	if stackArtifact.ProjectRoot != stackProject {
		t.Fatalf("unexpected stack project root: %s", stackArtifact.ProjectRoot)
	}

	cabalArtifact, ok := artifactAtPath(artifacts, filepath.Join(cabalProject, "dist-newstyle"))
	if !ok {
		t.Fatalf("expected dist-newstyle artifact")
	}
	if cabalArtifact.Kind != ArtifactHaskell {
		t.Fatalf("expected haskell kind, got %s", cabalArtifact.Kind)
	}
	if cabalArtifact.ProjectRoot != cabalProject {
		t.Fatalf("unexpected cabal project root: %s", cabalArtifact.ProjectRoot)
	}

	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, "dist-newstyle")); ok {
		t.Fatalf("unexpected orphan dist-newstyle artifact")
	}
}

func TestScanTerraformArtifactRequiresTerraformConfig(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "infra")
	testutil.MkdirAll(t, filepath.Join(project, ".terraform"))
	testutil.WriteFile(t, filepath.Join(project, "main.tf"), "terraform {}")

	orphan := filepath.Join(root, "orphan")
	testutil.MkdirAll(t, filepath.Join(orphan, ".terraform"))
	testutil.MkdirAll(t, filepath.Join(orphan, "main.tf"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})

	artifact, ok := artifactAtPath(artifacts, filepath.Join(project, ".terraform"))
	if !ok {
		t.Fatalf("expected terraform artifact")
	}
	if artifact.Kind != ArtifactTerraform {
		t.Fatalf("expected terraform kind, got %s", artifact.Kind)
	}
	if artifact.ProjectRoot != project {
		t.Fatalf("unexpected terraform project root: %s", artifact.ProjectRoot)
	}

	if _, ok := artifactAtPath(artifacts, filepath.Join(orphan, ".terraform")); ok {
		t.Fatalf("unexpected orphan terraform artifact")
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

func TestScanIncludesSymlinkedProjectsWhenRequested(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink requires privileges on windows")
	}

	work := t.TempDir()
	root := filepath.Join(work, "scan")
	testutil.MkdirAll(t, root)

	externalProject := filepath.Join(work, "external", "project")
	testutil.MkdirAll(t, filepath.Join(externalProject, "node_modules"))
	testutil.WriteFile(t, filepath.Join(externalProject, "package.json"), "{}")

	projectLink := filepath.Join(root, "project-link")
	testutil.Symlink(t, externalProject, projectLink)

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1, IncludeLinks: true})

	nodeArtifacts := artifactsByKind(artifacts, ArtifactNodeModule)
	if len(nodeArtifacts) != 1 {
		t.Fatalf("expected 1 node_modules artifact, got %d", len(nodeArtifacts))
	}

	expectedPath := filepath.Join(projectLink, "node_modules")
	if nodeArtifacts[0].Path != expectedPath {
		t.Fatalf("expected artifact path %s, got %s", expectedPath, nodeArtifacts[0].Path)
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

// --- New tests for uncovered paths ---

func TestScanEmptyRootPathReturnsError(t *testing.T) {
	_, _, err := Scan(context.Background(), ScanOptions{RootPath: "", MaxDepth: -1})
	if err == nil {
		t.Fatal("expected error for empty root path")
	}
}

func TestScanNonExistentRootReturnsError(t *testing.T) {
	_, _, err := Scan(context.Background(), ScanOptions{RootPath: "/nonexistent/path/xyz", MaxDepth: -1})
	if err == nil {
		t.Fatal("expected error for nonexistent root path")
	}
}

func TestScanFileAsRootReturnsError(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "file.txt")
	testutil.WriteFile(t, filePath, "not a dir")

	_, _, err := Scan(context.Background(), ScanOptions{RootPath: filePath, MaxDepth: -1})
	if err == nil {
		t.Fatal("expected error for file as root")
	}
}

func TestScanEmptyDirectoryReturnsNoArtifacts(t *testing.T) {
	root := t.TempDir()
	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})
	if len(artifacts) != 0 {
		t.Fatalf("expected 0 artifacts, got %d", len(artifacts))
	}
}

func TestScanAndMeasureIntegration(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "js")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")
	testutil.WriteFileSized(t, filepath.Join(project, "node_modules", "dep.js"), 100)

	artifacts, warnings, err := ScanAndMeasure(context.Background(), ScanOptions{RootPath: root, MaxDepth: -1})
	if err != nil {
		t.Fatalf("ScanAndMeasure failed: %v", err)
	}
	_ = warnings

	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].SizeBytes <= 0 {
		t.Fatalf("expected positive size, got %d", artifacts[0].SizeBytes)
	}
}

func TestScanAndMeasureEmptyRootError(t *testing.T) {
	_, _, err := ScanAndMeasure(context.Background(), ScanOptions{RootPath: "", MaxDepth: -1})
	if err == nil {
		t.Fatal("expected error for empty root")
	}
}

func TestScanAndMeasureSortsBySize(t *testing.T) {
	root := t.TempDir()

	small := filepath.Join(root, "small")
	testutil.MkdirAll(t, filepath.Join(small, "node_modules"))
	testutil.WriteFile(t, filepath.Join(small, "package.json"), "{}")
	testutil.WriteFileSized(t, filepath.Join(small, "node_modules", "s.js"), 10)

	big := filepath.Join(root, "big")
	testutil.MkdirAll(t, filepath.Join(big, "node_modules"))
	testutil.WriteFile(t, filepath.Join(big, "package.json"), "{}")
	testutil.WriteFileSized(t, filepath.Join(big, "node_modules", "b.js"), 1000)

	artifacts, _, err := ScanAndMeasure(context.Background(), ScanOptions{RootPath: root, MaxDepth: -1})
	if err != nil {
		t.Fatalf("ScanAndMeasure failed: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}
	if artifacts[0].SizeBytes < artifacts[1].SizeBytes {
		t.Fatal("expected sorted by size descending")
	}
}

func TestSortBySizeDesc(t *testing.T) {
	artifacts := []Artifact{
		{Path: "/a", SizeBytes: 100},
		{Path: "/b", SizeBytes: 500},
		{Path: "/c", SizeBytes: 200},
	}
	SortBySizeDesc(artifacts)
	if artifacts[0].SizeBytes != 500 {
		t.Fatalf("expected 500 first, got %d", artifacts[0].SizeBytes)
	}
	if artifacts[1].SizeBytes != 200 {
		t.Fatalf("expected 200 second, got %d", artifacts[1].SizeBytes)
	}
	if artifacts[2].SizeBytes != 100 {
		t.Fatalf("expected 100 third, got %d", artifacts[2].SizeBytes)
	}
}

func TestSortBySizeDescTiebreaker(t *testing.T) {
	artifacts := []Artifact{
		{Path: "/b", SizeBytes: 100},
		{Path: "/a", SizeBytes: 100},
	}
	SortBySizeDesc(artifacts)
	if artifacts[0].Path != "/a" {
		t.Fatalf("expected /a first on tie, got %s", artifacts[0].Path)
	}
}

func TestScanVirtualEnvWithPyprojectToml(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "python-proj")
	venv := filepath.Join(project, ".venv")
	testutil.MkdirAll(t, venv)
	testutil.WriteFile(t, filepath.Join(venv, "pyvenv.cfg"), "home = /usr/bin")
	testutil.WriteFile(t, filepath.Join(project, "pyproject.toml"), "[tool.poetry]")

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})
	venvArtifacts := artifactsByKind(artifacts, ArtifactPythonVenv)
	if len(venvArtifacts) != 1 {
		t.Fatalf("expected 1 venv artifact, got %d", len(venvArtifacts))
	}
	if venvArtifacts[0].ProjectRoot != project {
		t.Fatalf("expected project root %s, got %s", project, venvArtifacts[0].ProjectRoot)
	}
}

func TestScanCMakePrefixRule(t *testing.T) {
	root := t.TempDir()

	project := filepath.Join(root, "cmake-proj")
	testutil.MkdirAll(t, filepath.Join(project, "cmake-build-release"))
	testutil.WriteFile(t, filepath.Join(project, "CMakeLists.txt"), "cmake_minimum_required(VERSION 3.20)")

	// "cmake-build-" alone (no suffix) should not match
	testutil.MkdirAll(t, filepath.Join(project, "cmake-build-"))

	artifacts := mustScan(t, ScanOptions{RootPath: root, MaxDepth: -1})
	for _, a := range artifacts {
		if a.Path == filepath.Join(project, "cmake-build-") {
			t.Fatal("cmake-build- (empty suffix) should not match prefix rule")
		}
	}
}

func TestHasPathSuffixEmpty(t *testing.T) {
	if hasPathSuffix("/some/path", nil) {
		t.Fatal("expected false for empty suffix")
	}
}

func TestHasPathSuffixSingle(t *testing.T) {
	if !hasPathSuffix("/some/path/bundle", []string{"bundle"}) {
		t.Fatal("expected match for single element suffix")
	}
}

func TestHasPathSuffixMulti(t *testing.T) {
	if !hasPathSuffix("/some/vendor/bundle", []string{"vendor", "bundle"}) {
		t.Fatal("expected match for multi element suffix")
	}
	if hasPathSuffix("/some/other/bundle", []string{"vendor", "bundle"}) {
		t.Fatal("expected no match for wrong prefix")
	}
}

func TestIsWithinRoot(t *testing.T) {
	if !isWithinRoot("/a/b", "/a") {
		t.Fatal("expected /a/b within /a")
	}
	if !isWithinRoot("/a", "/a") {
		t.Fatal("expected /a within /a (self)")
	}
	if isWithinRoot("/b", "/a") {
		t.Fatal("expected /b not within /a")
	}
}

func TestSamePath(t *testing.T) {
	if !samePath("/a/b/../b", "/a/b") {
		t.Fatal("expected same path")
	}
	if samePath("/a", "/b") {
		t.Fatal("expected different path")
	}
}

func TestIsFile(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "test.txt")
	testutil.WriteFile(t, f, "hello")

	if !isFile(f) {
		t.Fatal("expected isFile to return true for file")
	}
	if isFile(root) {
		t.Fatal("expected isFile to return false for directory")
	}
	if isFile(filepath.Join(root, "nonexistent")) {
		t.Fatal("expected isFile to return false for nonexistent")
	}
}

func TestPathDistance(t *testing.T) {
	d := pathDistance("/a/b/c", "/a")
	if d != 2 {
		t.Fatalf("expected distance 2, got %d", d)
	}

	d = pathDistance("/a", "/a")
	if d != 0 {
		t.Fatalf("expected distance 0, got %d", d)
	}
}

func TestFindNearestAncestorNoMarker(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	testutil.MkdirAll(t, nested)

	ctx := newScanContext(root)
	_, ok := findNearestAncestorWithMarker(ctx, nested, []string{"nonexistent.marker"})
	if ok {
		t.Fatal("expected no match")
	}
}

func TestFindNearestAncestorWithGlobMarker(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "terraform")
	testutil.MkdirAll(t, project)
	testutil.WriteFile(t, filepath.Join(project, "providers.tf"), "terraform {}")

	ctx := newScanContext(root)
	markerRoot, ok := findNearestAncestorWithMarker(ctx, project, []string{"*.tf"})
	if !ok {
		t.Fatal("expected glob marker match")
	}
	if markerRoot != project {
		t.Fatalf("expected project root %s, got %s", project, markerRoot)
	}
}

func TestFindNearestAncestorOutsideRoot(t *testing.T) {
	ctx := newScanContext("/root")
	_, ok := findNearestAncestorWithMarker(ctx, "/outside", []string{"marker"})
	if ok {
		t.Fatal("expected no match outside root")
	}
}

func TestDetectArtifactNoMatch(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "random-dir")
	testutil.MkdirAll(t, dir)

	ctx := newScanContext(root)
	_, ok := detectArtifact(ctx, dir)
	if ok {
		t.Fatal("expected no match for random directory")
	}
}

func TestScanCollectsWarningsForUnreadableDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod not supported on windows")
	}

	root := t.TempDir()
	unreadable := filepath.Join(root, "noperm")
	testutil.MkdirAll(t, unreadable)

	project := filepath.Join(root, "js")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")

	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unreadable, 0o755) })

	artifacts, warnings, err := Scan(context.Background(), ScanOptions{RootPath: root, MaxDepth: -1})
	if err != nil {
		t.Fatalf("scan should not fail: %v", err)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for unreadable directory")
	}
	nodeArtifacts := artifactsByKind(artifacts, ArtifactNodeModule)
	if len(nodeArtifacts) != 1 {
		t.Fatalf("expected 1 node artifact despite warnings, got %d", len(nodeArtifacts))
	}
}

func TestScanAndMeasureWithWarnings(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "js")
	testutil.MkdirAll(t, filepath.Join(project, "node_modules"))
	testutil.WriteFile(t, filepath.Join(project, "package.json"), "{}")
	testutil.WriteFileSized(t, filepath.Join(project, "node_modules", "dep.js"), 50)

	artifacts, _, err := ScanAndMeasure(context.Background(), ScanOptions{RootPath: root, MaxDepth: -1})
	if err != nil {
		t.Fatalf("ScanAndMeasure failed: %v", err)
	}
	if len(artifacts) == 0 {
		t.Fatal("expected at least 1 artifact")
	}
}

func TestHasPathSuffixPartialMismatch(t *testing.T) {
	if hasPathSuffix("/a", []string{"x", "y"}) {
		t.Fatal("expected no match for short path with long suffix")
	}
}

func mustScan(t *testing.T, opts ScanOptions) []Artifact {
	t.Helper()
	artifacts, _, err := Scan(context.Background(), opts)
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
