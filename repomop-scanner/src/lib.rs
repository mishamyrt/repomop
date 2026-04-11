use std::collections::{HashMap, HashSet};
use std::fs;
use std::path::{Path, PathBuf};

use repomop_core::{
    ARTIFACT_DEFINITIONS, Artifact, ArtifactDefinition, ArtifactKind, ArtifactMatcher, ScanOptions,
    normalize_path, sort_artifacts_by_size_desc,
};
use repomop_fs::{SizeOptions, directories, recommended_worker_count};

#[derive(Debug, Clone)]
struct WalkItem {
    path: PathBuf,
    depth: usize,
}

#[derive(Debug)]
struct ScanContext {
    root: PathBuf,
    file_exists_cache: HashMap<PathBuf, bool>,
    marker_exists_cache: HashMap<(PathBuf, String), bool>,
}

impl ScanContext {
    fn new(root: PathBuf) -> Self {
        Self {
            root,
            file_exists_cache: HashMap::new(),
            marker_exists_cache: HashMap::new(),
        }
    }
}

#[derive(Debug, Clone)]
struct Candidate {
    kind: ArtifactKind,
    project_root: PathBuf,
    distance: usize,
}

pub fn scan(opts: &ScanOptions) -> Result<(Vec<Artifact>, Vec<String>), String> {
    if opts.root_path.as_os_str().is_empty() {
        return Err("root path is required".to_string());
    }

    let root = normalize_path(&opts.root_path);
    let root_metadata = fs::metadata(&root).map_err(|err| format!("stat root path: {err}"))?;
    if !root_metadata.is_dir() {
        return Err(format!("root path is not a directory: {}", root.display()));
    }

    let mut scan_ctx = ScanContext::new(root.clone());
    let mut stack = vec![WalkItem {
        path: root.clone(),
        depth: 0,
    }];
    let mut artifacts = Vec::new();
    let mut warnings = Vec::new();
    let mut visited_dirs = HashSet::new();

    while let Some(item) = stack.pop() {
        if opts.include_links {
            let real_path = match fs::canonicalize(&item.path) {
                Ok(path) => normalize_path(&path),
                Err(err) => {
                    warnings.push(format!("resolve directory {}: {err}", item.path.display()));
                    continue;
                }
            };
            if !visited_dirs.insert(real_path) {
                continue;
            }
        }

        let entries = match fs::read_dir(&item.path) {
            Ok(entries) => entries,
            Err(err) => {
                warnings.push(format!("read directory {}: {err}", item.path.display()));
                continue;
            }
        };

        let mut symlink_children = Vec::new();
        let mut regular_children = Vec::new();

        for entry in entries {
            let entry = match entry {
                Ok(entry) => entry,
                Err(err) => {
                    warnings.push(format!("read directory {}: {err}", item.path.display()));
                    continue;
                }
            };

            let child_path = entry.path();
            let (is_dir, is_symlink_dir) =
                match directory_entry_status(&child_path, opts.include_links, &entry.file_type()) {
                    Ok(status) => status,
                    Err(err) => {
                        warnings.push(format!("resolve directory {}: {err}", child_path.display()));
                        continue;
                    }
                };
            if !is_dir {
                continue;
            }

            let child_depth = item.depth + 1;
            if opts
                .max_depth
                .is_some_and(|max_depth| child_depth > max_depth)
            {
                continue;
            }

            if let Some(artifact) = detect_artifact(&mut scan_ctx, &child_path) {
                artifacts.push(artifact);
                continue;
            }

            let child = WalkItem {
                path: child_path,
                depth: child_depth,
            };
            if is_symlink_dir {
                symlink_children.push(child);
            } else {
                regular_children.push(child);
            }
        }

        stack.extend(symlink_children);
        stack.extend(regular_children);
    }

    Ok((artifacts, warnings))
}

pub fn scan_and_measure(opts: &ScanOptions) -> Result<(Vec<Artifact>, Vec<String>), String> {
    let (mut artifacts, mut warnings) = scan(opts)?;
    let paths: Vec<PathBuf> = artifacts
        .iter()
        .map(|artifact| artifact.path.clone())
        .collect();
    let (sizes, size_warnings) = directories(
        &paths,
        recommended_worker_count(),
        SizeOptions {
            include_links: opts.include_links,
        },
    );

    for artifact in &mut artifacts {
        artifact.size_bytes = sizes.get(&artifact.path).copied().unwrap_or(0);
    }

    warnings.extend(size_warnings);
    sort_artifacts_by_size_desc(&mut artifacts);
    Ok((artifacts, warnings))
}

fn directory_entry_status(
    path: &Path,
    include_links: bool,
    file_type: &std::io::Result<fs::FileType>,
) -> Result<(bool, bool), std::io::Error> {
    let file_type = match file_type {
        Ok(file_type) => *file_type,
        Err(err) => return Err(std::io::Error::new(err.kind(), err.to_string())),
    };

    if file_type.is_dir() {
        return Ok((true, false));
    }
    if !file_type.is_symlink() || !include_links {
        return Ok((false, false));
    }

    let metadata = fs::metadata(path)?;
    Ok((metadata.is_dir(), true))
}

fn detect_artifact(ctx: &mut ScanContext, path: &Path) -> Option<Artifact> {
    let mut best: Option<Candidate> = None;

    for definition in ARTIFACT_DEFINITIONS {
        if let Some(candidate) = detect_candidate(ctx, definition, path) {
            update_best(&mut best, candidate);
        }
    }

    best.map(|candidate| Artifact {
        kind: candidate.kind,
        path: path.to_path_buf(),
        project_root: candidate.project_root,
        size_bytes: 0,
    })
}

fn detect_candidate(
    ctx: &mut ScanContext,
    definition: &ArtifactDefinition,
    path: &Path,
) -> Option<Candidate> {
    if !definition_matches(ctx, definition, path) {
        return None;
    }

    let project_root = match definition.matcher {
        ArtifactMatcher::VirtualEnv => {
            infer_venv_project_root(ctx, path, definition.project_markers)
        }
        _ => find_nearest_ancestor_with_marker(
            ctx,
            path.parent().unwrap_or(path),
            definition.project_markers,
        )?,
    };

    Some(Candidate {
        kind: definition.kind,
        distance: path_distance(path, &project_root),
        project_root,
    })
}

fn definition_matches(ctx: &mut ScanContext, definition: &ArtifactDefinition, path: &Path) -> bool {
    let Some(base) = path.file_name().and_then(|name| name.to_str()) else {
        return false;
    };

    match definition.matcher {
        ArtifactMatcher::NamedDir(name) => base == name,
        ArtifactMatcher::PrefixDir(prefix) => base.starts_with(prefix) && base.len() > prefix.len(),
        ArtifactMatcher::PathSuffix(parts) => has_path_suffix(path, parts),
        ArtifactMatcher::VirtualEnv => is_virtual_env(ctx, path),
    }
}

fn update_best(best: &mut Option<Candidate>, candidate: Candidate) {
    match best {
        Some(current) if current.distance <= candidate.distance => {}
        _ => *best = Some(candidate),
    }
}

fn is_virtual_env(ctx: &mut ScanContext, path: &Path) -> bool {
    cached_is_file(ctx, &path.join("pyvenv.cfg"))
        || (cached_is_file(ctx, &path.join("bin/activate"))
            && cached_is_file(ctx, &path.join("bin/python")))
}

fn infer_venv_project_root(
    ctx: &mut ScanContext,
    path: &Path,
    project_markers: &[&str],
) -> PathBuf {
    find_nearest_ancestor_with_marker(ctx, path.parent().unwrap_or(path), project_markers)
        .unwrap_or_else(|| path.parent().unwrap_or(path).to_path_buf())
}

fn find_nearest_ancestor_with_marker(
    ctx: &mut ScanContext,
    start: &Path,
    markers: &[&str],
) -> Option<PathBuf> {
    let mut current = normalize_path(start);

    loop {
        if !is_within_root(&current, &ctx.root) {
            return None;
        }

        if markers
            .iter()
            .any(|marker| cached_has_marker(ctx, &current, marker))
        {
            return Some(current);
        }

        if current == ctx.root {
            return None;
        }
        let Some(parent) = current.parent() else {
            return None;
        };
        current = parent.to_path_buf();
    }
}

fn cached_has_marker(ctx: &mut ScanContext, dir: &Path, marker: &str) -> bool {
    let key = (dir.to_path_buf(), marker.to_string());
    if let Some(value) = ctx.marker_exists_cache.get(&key) {
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
                        && cached_is_file(ctx, &path)
                }),
            Err(_) => false,
        }
    } else {
        cached_is_file(ctx, &dir.join(marker))
    };

    ctx.marker_exists_cache.insert(key, result);
    result
}

fn cached_is_file(ctx: &mut ScanContext, path: &Path) -> bool {
    if let Some(value) = ctx.file_exists_cache.get(path) {
        return *value;
    }

    let result = fs::metadata(path)
        .map(|metadata| metadata.is_file())
        .unwrap_or(false);
    ctx.file_exists_cache.insert(path.to_path_buf(), result);
    result
}

fn wildcard_match(name: &str, pattern: &str) -> bool {
    let name = name.as_bytes();
    let pattern = pattern.as_bytes();
    let (mut text_idx, mut pattern_idx) = (0usize, 0usize);
    let mut star_idx = None;
    let mut match_idx = 0usize;

    while text_idx < name.len() {
        if pattern_idx < pattern.len()
            && (pattern[pattern_idx] == b'?' || pattern[pattern_idx] == name[text_idx])
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

fn has_path_suffix(path: &Path, suffix: &[&str]) -> bool {
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

fn path_distance(path: &Path, ancestor: &Path) -> usize {
    path.strip_prefix(ancestor)
        .map(|relative| relative.components().count())
        .unwrap_or(0)
}

fn is_within_root(path: &Path, root: &Path) -> bool {
    path == root || path.starts_with(root)
}

#[cfg(test)]
mod tests {
    use std::fs;

    use repomop_core::{ArtifactKind, ScanOptions};
    use tempfile::TempDir;

    use super::{scan, scan_and_measure};

    #[cfg(unix)]
    fn symlink_dir(target: &std::path::Path, link: &std::path::Path) {
        std::os::unix::fs::symlink(target, link).unwrap();
    }

    fn make_opts(root: &std::path::Path) -> ScanOptions {
        ScanOptions {
            root_path: root.to_path_buf(),
            max_depth: None,
            include_links: false,
        }
    }

    #[test]
    fn finds_node_modules_only_with_package_json() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("js-project");
        let orphan = temp.path().join("orphan");
        fs::create_dir_all(project.join("node_modules")).unwrap();
        fs::create_dir_all(orphan.join("node_modules")).unwrap();
        fs::write(project.join("package.json"), "{}").unwrap();

        let (artifacts, _) = scan(&make_opts(temp.path())).unwrap();
        assert_eq!(artifacts.len(), 1);
        assert_eq!(artifacts[0].kind, ArtifactKind::NodeModules);
        assert_eq!(artifacts[0].path, project.join("node_modules"));
        assert_eq!(artifacts[0].project_root, project);
    }

    #[test]
    fn distinguishes_rust_and_maven_targets() {
        let temp = TempDir::new().unwrap();
        let rust_project = temp.path().join("rust");
        let maven_project = temp.path().join("maven");
        fs::create_dir_all(rust_project.join("target")).unwrap();
        fs::create_dir_all(maven_project.join("target")).unwrap();
        fs::write(rust_project.join("Cargo.toml"), "[package]\nname='x'\n").unwrap();
        fs::write(maven_project.join("pom.xml"), "<project/>").unwrap();

        let (artifacts, _) = scan(&make_opts(temp.path())).unwrap();
        assert!(artifacts.iter().any(|artifact| {
            artifact.kind == ArtifactKind::RustTarget
                && artifact.path == rust_project.join("target")
        }));
        assert!(artifacts.iter().any(|artifact| {
            artifact.kind == ArtifactKind::JavaMaven
                && artifact.path == maven_project.join("target")
        }));
    }

    #[test]
    fn build_uses_nearest_project_marker() {
        let temp = TempDir::new().unwrap();
        let flutter_project = temp.path().join("apps/flutter-app");
        fs::create_dir_all(flutter_project.join("build")).unwrap();
        fs::write(
            temp.path().join("settings.gradle"),
            "rootProject.name='mono'",
        )
        .unwrap();
        fs::write(flutter_project.join("pubspec.yaml"), "name: app").unwrap();

        let (artifacts, _) = scan(&make_opts(temp.path())).unwrap();
        let artifact = artifacts
            .iter()
            .find(|artifact| artifact.path == flutter_project.join("build"))
            .unwrap();
        assert_eq!(artifact.kind, ArtifactKind::DartFlutter);
        assert_eq!(artifact.project_root, flutter_project);
    }

    #[test]
    fn detects_virtualenvs() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("python");
        let venv = project.join("custom_env");
        fs::create_dir_all(&venv).unwrap();
        fs::write(project.join("pyproject.toml"), "[project]\nname='x'").unwrap();
        fs::write(venv.join("pyvenv.cfg"), "home = /usr/bin").unwrap();

        let (artifacts, _) = scan(&make_opts(temp.path())).unwrap();
        assert_eq!(artifacts.len(), 1);
        assert_eq!(artifacts[0].kind, ArtifactKind::PythonVenv);
        assert_eq!(artifacts[0].path, venv);
        assert_eq!(artifacts[0].project_root, project);
    }

    #[test]
    fn respects_max_depth() {
        let temp = TempDir::new().unwrap();
        let nested = temp.path().join("a/b/c");
        fs::create_dir_all(nested.join("node_modules")).unwrap();
        fs::write(nested.join("package.json"), "{}").unwrap();

        let mut shallow = make_opts(temp.path());
        shallow.max_depth = Some(2);
        let (artifacts, _) = scan(&shallow).unwrap();
        assert!(artifacts.is_empty());

        let mut deep = make_opts(temp.path());
        deep.max_depth = Some(4);
        let (artifacts, _) = scan(&deep).unwrap();
        assert_eq!(artifacts.len(), 1);
    }

    #[test]
    #[cfg(unix)]
    fn skips_symlinked_projects_by_default() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        fs::create_dir_all(project.join("node_modules")).unwrap();
        fs::write(project.join("package.json"), "{}").unwrap();
        symlink_dir(&project, &temp.path().join("project-link"));

        let (artifacts, _) = scan(&make_opts(temp.path())).unwrap();
        assert_eq!(artifacts.len(), 1);
    }

    #[test]
    #[cfg(unix)]
    fn includes_symlinked_projects_when_requested() {
        let temp = TempDir::new().unwrap();
        let root = temp.path().join("scan");
        let external_project = temp.path().join("external/project");
        fs::create_dir_all(root.as_path()).unwrap();
        fs::create_dir_all(external_project.join("node_modules")).unwrap();
        fs::write(external_project.join("package.json"), "{}").unwrap();
        symlink_dir(&external_project, &root.join("project-link"));

        let mut opts = make_opts(&root);
        opts.include_links = true;
        let (artifacts, _) = scan(&opts).unwrap();
        assert_eq!(artifacts.len(), 1);
        assert_eq!(artifacts[0].path, root.join("project-link/node_modules"));
    }

    #[test]
    fn scan_and_measure_sorts_by_size_desc() {
        let temp = TempDir::new().unwrap();
        let rust_project = temp.path().join("rust");
        let js_project = temp.path().join("js");

        fs::create_dir_all(rust_project.join("target")).unwrap();
        fs::create_dir_all(js_project.join("node_modules")).unwrap();
        fs::write(rust_project.join("Cargo.toml"), "[package]\nname='x'\n").unwrap();
        fs::write(js_project.join("package.json"), "{}").unwrap();
        fs::write(rust_project.join("target/a.bin"), [0u8; 20]).unwrap();
        fs::write(js_project.join("node_modules/a.js"), [0u8; 10]).unwrap();

        let (artifacts, _) = scan_and_measure(&make_opts(temp.path())).unwrap();
        assert_eq!(artifacts.len(), 2);
        assert!(artifacts[0].size_bytes >= artifacts[1].size_bytes);
    }
}
