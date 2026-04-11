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

pub fn relative_path_or_self(root: &Path, path: &Path) -> PathBuf {
    if root.as_os_str().is_empty() || path.as_os_str().is_empty() {
        return path.to_path_buf();
    }

    match path.strip_prefix(root) {
        Ok(relative) if !relative.as_os_str().is_empty() => relative.to_path_buf(),
        _ => path.to_path_buf(),
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
        assert_eq!(relative_path_or_self(root, root), PathBuf::from("/root"));
    }

    #[test]
    fn relative_returns_child() {
        let root = Path::new("/repo");
        let path = Path::new("/repo/src/main.rs");
        assert_eq!(relative_path_or_self(root, path), PathBuf::from("src/main.rs"));
    }
}
