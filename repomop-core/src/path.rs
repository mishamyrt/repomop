use std::borrow::Cow;
use std::path::{Component, Path, PathBuf};

pub fn normalize_path(path: &Path) -> PathBuf {
    let mut normalized = PathBuf::new();

    for component in path.components() {
        match component {
            Component::CurDir => {}
            Component::ParentDir => {
                normalized.pop();
            }
            Component::RootDir | Component::Prefix(_) | Component::Normal(_) => {
                normalized.push(component.as_os_str());
            }
        }
    }

    if normalized.as_os_str().is_empty() { PathBuf::from(".") } else { normalized }
}

/// Returns the path relative to `root`, or the original path unchanged if it is
/// not a descendant of `root`. Never allocates when the path is returned as-is.
pub fn relative_path_or_self<'a>(root: &Path, path: &'a Path) -> Cow<'a, Path> {
    if root.as_os_str().is_empty() || path.as_os_str().is_empty() {
        return Cow::Borrowed(path);
    }

    match path.strip_prefix(root) {
        Ok(relative) if !relative.as_os_str().is_empty() => Cow::Borrowed(relative),
        _ => Cow::Borrowed(path),
    }
}

#[cfg(test)]
mod tests {
    use std::path::{Path, PathBuf};

    use super::{normalize_path, relative_path_or_self};

    #[test]
    fn normalize_keeps_absolute_shape() {
        let path = Path::new("/tmp/../repo/./app");
        assert_eq!(normalize_path(path), PathBuf::from("/repo/app"));
    }

    #[test]
    fn relative_returns_original_for_same_path() {
        let root = Path::new("/root");
        assert_eq!(relative_path_or_self(root, root).as_ref(), Path::new("/root"));
    }

    #[test]
    fn relative_returns_child() {
        let root = Path::new("/repo");
        let path = Path::new("/repo/src/main.rs");
        assert_eq!(
            relative_path_or_self(root, path).as_ref(),
            Path::new("src/main.rs")
        );
    }
}
