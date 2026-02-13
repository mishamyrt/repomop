package scanner

import (
	"os"
	"path/filepath"
	"strings"
)

func detectArtifact(path string, root string) (Artifact, bool) {
	var (
		matched      bool
		bestArtifact Artifact
		bestDistance int
	)

	for _, rule := range defaultArtifactRules {
		if !rule.match(path) {
			continue
		}

		match, ok := rule.resolveProjectRoot(path, root)
		if !ok {
			continue
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

	if matched {
		return bestArtifact, true
	}

	return Artifact{}, false
}

func isVirtualEnv(path string) bool {
	if isFile(filepath.Join(path, "pyvenv.cfg")) {
		return true
	}

	activatePath := filepath.Join(path, "bin", "activate")
	pythonPath := filepath.Join(path, "bin", "python")
	return isFile(activatePath) && isFile(pythonPath)
}

func inferVenvProjectRoot(venvPath string, root string) string {
	markers := []string{"pyproject.toml", "requirements.txt", "setup.py", "Pipfile"}
	if projectRoot, ok := findNearestAncestorWithMarker(filepath.Dir(venvPath), root, markers); ok {
		return projectRoot
	}
	return filepath.Dir(venvPath)
}

func findNearestAncestorWithMarker(start string, root string, markers []string) (string, bool) {
	curr := filepath.Clean(start)
	root = filepath.Clean(root)

	for {
		if !isWithinRoot(curr, root) {
			return "", false
		}
		for _, marker := range markers {
			if isFile(filepath.Join(curr, marker)) {
				return curr, true
			}
		}
		if samePath(curr, root) {
			return "", false
		}
		parent := filepath.Dir(curr)
		if samePath(parent, curr) {
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
	return rel == "." || (rel != "" && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..")
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
