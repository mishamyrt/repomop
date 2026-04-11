use std::path::Path;

pub(crate) fn wildcard_match(name: &str, pattern: &str) -> bool {
    let name = name.as_bytes();
    let pattern = pattern.as_bytes();
    let (mut text_idx, mut pattern_idx) = (0usize, 0usize);
    let mut star_idx = None;
    let mut match_idx = 0usize;

    while text_idx < name.len() {
        if pattern_idx < pattern.len()
            && (pattern[pattern_idx] == b'?'
                || pattern[pattern_idx] == name[text_idx])
        {
            text_idx += 1;
            pattern_idx += 1;
        } else if pattern_idx < pattern.len() && pattern[pattern_idx] == b'*' {
            star_idx = Some(pattern_idx);
            match_idx = text_idx;
            pattern_idx += 1;
        } else if let Some(star) = star_idx {
            pattern_idx = star + 1;
            match_idx += 1;
            text_idx = match_idx;
        } else {
            return false;
        }
    }

    while pattern_idx < pattern.len() && pattern[pattern_idx] == b'*' {
        pattern_idx += 1;
    }

    pattern_idx == pattern.len()
}

pub(crate) fn has_path_suffix(path: &Path, suffix: &[&str]) -> bool {
    let mut current = path;
    for expected in suffix.iter().rev() {
        let Some(name) = current.file_name().and_then(|name| name.to_str()) else {
            return false;
        };
        if name != *expected {
            return false;
        }
        let Some(parent) = current.parent() else {
            return false;
        };
        current = parent;
    }
    true
}

pub(crate) fn path_distance(path: &Path, ancestor: &Path) -> usize {
    path.strip_prefix(ancestor)
        .map(|relative| relative.components().count())
        .unwrap_or(0)
}

pub(crate) fn is_within_root(path: &Path, root: &Path) -> bool {
    path == root || path.starts_with(root)
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use super::{has_path_suffix, is_within_root, path_distance, wildcard_match};

    #[test]
    fn wildcard_match_supports_exact_question_and_star() {
        assert!(wildcard_match("demo.cabal", "*.cabal"));
        assert!(wildcard_match("ab", "a?"));
        assert!(wildcard_match("cmake-build-debug", "cmake-build-*"));
        assert!(!wildcard_match("Cargo.lock", "*.toml"));
        assert!(!wildcard_match("ab", "a"));
    }

    #[test]
    fn has_path_suffix_matches_trailing_components() {
        let path = Path::new("/tmp/project/vendor/bundle");

        assert!(has_path_suffix(path, &["vendor", "bundle"]));
        assert!(has_path_suffix(path, &["bundle"]));
        assert!(!has_path_suffix(path, &["tmp", "project"]));
        assert!(!has_path_suffix(path, &["vendor", "cache"]));
    }

    #[test]
    fn path_distance_counts_relative_components() {
        let path = Path::new("/tmp/project/a/b");
        let ancestor = Path::new("/tmp/project");

        assert_eq!(path_distance(path, ancestor), 2);
        assert_eq!(path_distance(ancestor, ancestor), 0);
        assert_eq!(path_distance(path, Path::new("/outside")), 0);
    }

    #[test]
    fn is_within_root_accepts_root_and_descendants_only() {
        let root = Path::new("/tmp/project");

        assert!(is_within_root(root, root));
        assert!(is_within_root(Path::new("/tmp/project/src"), root));
        assert!(!is_within_root(Path::new("/tmp/project-sibling"), root));
    }
}
