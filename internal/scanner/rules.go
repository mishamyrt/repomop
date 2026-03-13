package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

// scanContext holds per-scan state shared across detectArtifact calls:
// a rule index for O(1) name-based lookup and a stat cache to avoid
// repeated os.Stat calls for the same marker files.
type scanContext struct {
	root            string
	fileExistsCache map[string]bool
	ruleIndex       map[string][]artifactRule
	specialRules    []artifactRule
}

func newScanContext(root string) *scanContext {
	idx := make(map[string][]artifactRule)
	var special []artifactRule
	for _, rule := range defaultArtifactRules {
		if rule.dirName != "" {
			idx[rule.dirName] = append(idx[rule.dirName], rule)
		} else {
			special = append(special, rule)
		}
	}
	return &scanContext{
		root:            root,
		fileExistsCache: make(map[string]bool, 256),
		ruleIndex:       idx,
		specialRules:    special,
	}
}

func detectArtifact(ctx *scanContext, path string) (Artifact, bool) {
	var (
		matched      bool
		bestArtifact Artifact
		bestDistance int
	)

	base := filepath.Base(path)
	candidates := ctx.ruleIndex[base]

	runRule := func(rule artifactRule) {
		if !rule.match(ctx, path) {
			return
		}
		match, ok := rule.resolveProjectRoot(ctx, path)
		if !ok {
			return
		}
		if !matched || match.distance < bestDistance {
			matched = true
			bestDistance = match.distance
			bestArtifact = Artifact{
				Kind:        rule.kind,
				Path:        path,
				ProjectRoot: match.projectRoot,
			}
		}
	}

	for _, rule := range candidates {
		runRule(rule)
	}
	for _, rule := range ctx.specialRules {
		runRule(rule)
	}

	if matched {
		return bestArtifact, true
	}

	return Artifact{}, false
}

func isVirtualEnv(ctx *scanContext, path string) bool {
	if cachedIsFile(ctx, filepath.Join(path, "pyvenv.cfg")) {
		return true
	}
	activatePath := filepath.Join(path, "bin", "activate")
	pythonPath := filepath.Join(path, "bin", "python")
	return cachedIsFile(ctx, activatePath) && cachedIsFile(ctx, pythonPath)
}

func inferVenvProjectRoot(ctx *scanContext, venvPath string) string {
	markers := []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"}
	if projectRoot, ok := findNearestAncestorWithMarker(ctx, filepath.Dir(venvPath), markers); ok {
		return projectRoot
	}
	return filepath.Dir(venvPath)
}

func findNearestAncestorWithMarker(ctx *scanContext, start string, markers []string) (string, bool) {
	curr := filepath.Clean(start)
	root := ctx.root

	for {
		if !isWithinRoot(curr, root) {
			return "", false
		}
		for _, marker := range markers {
			if cachedIsFile(ctx, filepath.Join(curr, marker)) {
				return curr, true
			}
		}
		if curr == root {
			return "", false
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			return "", false
		}
		curr = parent
	}
}

func isWithinRoot(path string, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}

func cachedIsFile(ctx *scanContext, path string) bool {
	if v, ok := ctx.fileExistsCache[path]; ok {
		return v
	}
	info, err := os.Stat(path)
	result := err == nil && !info.IsDir()
	ctx.fileExistsCache[path] = result
	return result
}

func samePath(a string, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b)
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
