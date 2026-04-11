use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};

use crate::matchers::wildcard_match;

#[derive(Debug)]
pub(crate) struct ScanContext {
    root: PathBuf,
    file_exists_cache: HashMap<PathBuf, bool>,
    marker_exists_cache: HashMap<(PathBuf, String), bool>,
}

impl ScanContext {
    pub(crate) fn new(root: PathBuf) -> Self {
        Self {
            root,
            file_exists_cache: HashMap::new(),
            marker_exists_cache: HashMap::new(),
        }
    }

    pub(crate) fn root(&self) -> &Path {
        &self.root
    }

    pub(crate) fn has_marker(&mut self, dir: &Path, marker: &str) -> bool {
        let key = (dir.to_path_buf(), marker.to_string());
        if let Some(value) = self.marker_exists_cache.get(&key) {
            return *value;
        }

        let result = if marker.contains('*') {
            match fs::read_dir(dir) {
                Ok(entries) => entries
                    .filter_map(Result::ok)
                    .map(|entry| entry.path())
                    .any(|path| {
                        path.file_name()
                            .and_then(|name| name.to_str())
                            .is_some_and(|name| wildcard_match(name, marker))
                            && self.is_file(&path)
                    }),
                Err(_) => false,
            }
        } else {
            self.is_file(&dir.join(marker))
        };

        self.marker_exists_cache.insert(key, result);
        result
    }

    pub(crate) fn is_file(&mut self, path: &Path) -> bool {
        if let Some(value) = self.file_exists_cache.get(path) {
            return *value;
        }

        let result =
            fs::metadata(path).map(|metadata| metadata.is_file()).unwrap_or(false);
        self.file_exists_cache.insert(path.to_path_buf(), result);
        result
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::ScanContext;

    #[test]
    fn new_keeps_root_path() {
        let temp = TempDir::new().unwrap();
        let ctx = ScanContext::new(temp.path().to_path_buf());

        assert_eq!(ctx.root(), temp.path());
    }

    #[test]
    fn is_file_distinguishes_files_and_directories() {
        let temp = TempDir::new().unwrap();
        let file = temp.path().join("file.txt");
        let dir = temp.path().join("dir");
        fs::write(&file, "x").unwrap();
        fs::create_dir(&dir).unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        assert!(ctx.is_file(&file));
        assert!(!ctx.is_file(&dir));
        assert!(!ctx.is_file(&temp.path().join("missing.txt")));
    }

    #[test]
    fn has_marker_matches_exact_file_name() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        fs::create_dir(&project).unwrap();
        fs::write(project.join("Cargo.toml"), "[package]").unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        assert!(ctx.has_marker(&project, "Cargo.toml"));
        assert!(!ctx.has_marker(&project, "package.json"));
    }

    #[test]
    fn has_marker_matches_wildcards_only_for_files() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        fs::create_dir(&project).unwrap();
        fs::write(project.join("demo.cabal"), "name: demo").unwrap();
        fs::create_dir(project.join("nested.cabal")).unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        assert!(ctx.has_marker(&project, "*.cabal"));
    }

    #[test]
    fn has_marker_returns_false_for_missing_directory() {
        let temp = TempDir::new().unwrap();
        let mut ctx = ScanContext::new(temp.path().to_path_buf());

        assert!(!ctx.has_marker(&temp.path().join("missing"), "*.tf"));
    }
}
