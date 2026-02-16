package pathutil

import "path/filepath"

// RelativePathOrSelf returns the path relative to root, or the original path
// if it cannot be made relative or equals root.
func RelativePathOrSelf(root, path string) string {
	if root == "" || path == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	if rel == "." {
		return path
	}
	return rel
}
