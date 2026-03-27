package scanner

import (
	"path/filepath"
	"strings"
)

type ruleMatch struct {
	projectRoot string
	distance    int
}

type artifactRule struct {
	kind               ArtifactKind
	dirName            string
	match              func(ctx *scanContext, path string) bool
	resolveProjectRoot func(ctx *scanContext, path string) (ruleMatch, bool)
}

var gradleMarkers = []string{"settings.gradle", "settings.gradle.kts", "build.gradle", "build.gradle.kts"}
var haskellStackMarkers = []string{"stack.yaml", "*.cabal"}
var haskellCabalMarkers = []string{"*.cabal", "cabal.project"}
var terraformMarkers = []string{"*.tf", "*.tf.json", ".terraform.lock.hcl"}

var defaultArtifactRules = []artifactRule{
	namedDirRule(ArtifactNodeModule, "node_modules", []string{"package.json"}),
	namedDirRule(ArtifactRustTarget, "target", []string{"Cargo.toml"}),
	namedDirRule(ArtifactJavaMaven, "target", []string{"pom.xml"}),
	namedDirRule(ArtifactSwiftBuild, ".build", []string{"Package.swift"}),
	namedDirRule(ArtifactElixir, "_build", []string{"mix.exs"}),
	namedDirRule(ArtifactElixir, "deps", []string{"mix.exs"}),
	namedDirRule(ArtifactHaskell, ".stack-work", haskellStackMarkers),
	namedDirRule(ArtifactHaskell, "dist-newstyle", haskellCabalMarkers),
	namedDirRule(ArtifactTerraform, ".terraform", terraformMarkers),

	namedDirRule(ArtifactJavaGradle, ".gradle", gradleMarkers),
	namedDirRule(ArtifactJavaGradle, "build", gradleMarkers),
	namedDirRule(ArtifactJavaGradle, "out", gradleMarkers),

	namedDirRule(ArtifactCMake, "build", []string{"CMakeLists.txt"}),
	prefixDirRule(ArtifactCMake, "cmake-build-", []string{"CMakeLists.txt"}),
	namedDirRule(ArtifactCMake, "CMakeFiles", []string{"CMakeLists.txt"}),

	namedDirRule(ArtifactFlutter, ".dart_tool", []string{"pubspec.yaml"}),
	namedDirRule(ArtifactFlutter, "build", []string{"pubspec.yaml"}),

	namedDirRule(ArtifactRuby, ".bundle", []string{"Gemfile"}),
	pathSuffixRule(ArtifactRuby, []string{"vendor", "bundle"}, []string{"Gemfile"}),

	namedDirRule(ArtifactPHP, "vendor", []string{"composer.json"}),

	namedDirRule(ArtifactZig, "zig-out", []string{"build.zig"}),
	namedDirRule(ArtifactZig, ".zig-cache", []string{"build.zig"}),

	namedDirRule(ArtifactPlatformIO, ".pio", []string{"platformio.ini"}),

	pythonVenvRule(),
}

func namedDirRule(kind ArtifactKind, dirName string, projectMarkers []string) artifactRule {
	return artifactRule{
		kind:    kind,
		dirName: dirName,
		match: func(_ *scanContext, path string) bool {
			return filepath.Base(path) == dirName
		},
		resolveProjectRoot: func(ctx *scanContext, path string) (ruleMatch, bool) {
			return markerMatch(ctx, path, projectMarkers)
		},
	}
}

func prefixDirRule(kind ArtifactKind, prefix string, projectMarkers []string) artifactRule {
	return artifactRule{
		kind: kind,
		match: func(_ *scanContext, path string) bool {
			base := filepath.Base(path)
			return strings.HasPrefix(base, prefix) && len(base) > len(prefix)
		},
		resolveProjectRoot: func(ctx *scanContext, path string) (ruleMatch, bool) {
			return markerMatch(ctx, path, projectMarkers)
		},
	}
}

func pathSuffixRule(kind ArtifactKind, suffix []string, projectMarkers []string) artifactRule {
	return artifactRule{
		kind: kind,
		match: func(_ *scanContext, path string) bool {
			return hasPathSuffix(path, suffix)
		},
		resolveProjectRoot: func(ctx *scanContext, path string) (ruleMatch, bool) {
			return markerMatch(ctx, path, projectMarkers)
		},
	}
}

func pythonVenvRule() artifactRule {
	return artifactRule{
		kind: ArtifactPythonVenv,
		match: func(ctx *scanContext, path string) bool {
			return isVirtualEnv(ctx, path)
		},
		resolveProjectRoot: func(ctx *scanContext, path string) (ruleMatch, bool) {
			projectRoot := inferVenvProjectRoot(ctx, path)
			return ruleMatch{
				projectRoot: projectRoot,
				distance:    pathDistance(path, projectRoot),
			}, true
		},
	}
}

func markerMatch(ctx *scanContext, path string, markers []string) (ruleMatch, bool) {
	projectRoot, ok := findNearestAncestorWithMarker(ctx, filepath.Dir(path), markers)
	if !ok {
		return ruleMatch{}, false
	}
	return ruleMatch{
		projectRoot: projectRoot,
		distance:    pathDistance(path, projectRoot),
	}, true
}

func pathDistance(path string, ancestor string) int {
	rel, err := filepath.Rel(ancestor, path)
	if err != nil || rel == "." {
		return 0
	}
	parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
	distance := 0
	for _, part := range parts {
		if part != "" && part != "." {
			distance++
		}
	}
	return distance
}

func hasPathSuffix(path string, suffix []string) bool {
	if len(suffix) == 0 {
		return false
	}

	curr := filepath.Clean(path)
	for i := len(suffix) - 1; i >= 0; i-- {
		if filepath.Base(curr) != suffix[i] {
			return false
		}
		if i == 0 {
			return true
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			return false
		}
		curr = parent
	}
	return true
}
