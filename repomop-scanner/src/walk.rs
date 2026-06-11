use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};

use repomop_core::{
    Artifact, ScanOptions, normalize_path, sort_artifacts_by_size_desc,
};
use repomop_fs::{SizeOptions, directories, recommended_worker_count};

use crate::context::ScanContext;
use crate::detect::{detect_artifact, detect_virtual_env_artifact};

#[derive(Debug, Clone)]
struct WalkItem {
    path: PathBuf,
    depth: usize,
}

pub(crate) fn scan(
    opts: &ScanOptions,
) -> Result<(Vec<Artifact>, Vec<String>), String> {
    if opts.root_path.as_os_str().is_empty() {
        return Err("root path is required".to_string());
    }

    let root = normalize_path(&opts.root_path);
    let root_metadata =
        fs::metadata(&root).map_err(|err| format!("stat root path: {err}"))?;
    if !root_metadata.is_dir() {
        return Err(format!("root path is not a directory: {}", root.display()));
    }

    let mut scan_ctx = ScanContext::new(root.clone());
    let mut stack = vec![WalkItem { path: root, depth: 0 }];
    let mut artifacts = Vec::new();
    let mut warnings = Vec::new();
    let mut visited_dirs = HashSet::new();

    while let Some(item) = stack.pop() {
        if opts.include_links {
            let real_path = match fs::canonicalize(&item.path) {
                Ok(path) => normalize_path(&path),
                Err(err) => {
                    warnings.push(format!(
                        "resolve directory {}: {err}",
                        item.path.display()
                    ));
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
                warnings
                    .push(format!("read directory {}: {err}", item.path.display()));
                continue;
            }
        };

        let mut symlink_children = Vec::new();
        let mut regular_children = Vec::new();
        let mut has_pyvenv_cfg = false;
        let mut bin_path = None;

        for entry in entries {
            let entry = match entry {
                Ok(entry) => entry,
                Err(err) => {
                    warnings.push(format!(
                        "read directory {}: {err}",
                        item.path.display()
                    ));
                    continue;
                }
            };

            let child_path = entry.path();
            let child_name = entry.file_name();
            let child_name = child_name.to_str();
            let file_type = entry.file_type();

            if child_name == Some("pyvenv.cfg")
                && file_type.as_ref().is_ok_and(fs::FileType::is_file)
            {
                has_pyvenv_cfg = true;
            }
            if child_name == Some("bin") {
                bin_path = Some(child_path.clone());
            }

            let (is_dir, is_symlink_dir) = match directory_entry_status(
                &child_path,
                opts.include_links,
                &file_type,
            ) {
                Ok(status) => status,
                Err(err) => {
                    warnings.push(format!(
                        "resolve directory {}: {err}",
                        child_path.display()
                    ));
                    continue;
                }
            };
            if !is_dir {
                continue;
            }

            let child_depth = item.depth + 1;
            if opts.max_depth.is_some_and(|max_depth| child_depth > max_depth) {
                continue;
            }

            if let Some(artifact) = detect_artifact(&mut scan_ctx, &child_path) {
                artifacts.push(artifact);
                continue;
            }

            let child = WalkItem { path: child_path, depth: child_depth };
            if is_symlink_dir {
                symlink_children.push(child);
            } else {
                regular_children.push(child);
            }
        }

        if item.depth > 0
            && (has_pyvenv_cfg
                || bin_path.as_deref().is_some_and(has_bin_python_markers))
        {
            if let Some(artifact) =
                detect_virtual_env_artifact(&mut scan_ctx, &item.path)
            {
                artifacts.push(artifact);
                continue;
            }
        }

        stack.extend(symlink_children);
        stack.extend(regular_children);
    }

    Ok((artifacts, warnings))
}

pub(crate) fn scan_and_measure(
    opts: &ScanOptions,
) -> Result<(Vec<Artifact>, Vec<String>), String> {
    let (mut artifacts, mut warnings) = scan(opts)?;
    let paths: Vec<PathBuf> =
        artifacts.iter().map(|artifact| artifact.path.clone()).collect();
    let (sizes, size_warnings) = directories(
        &paths,
        recommended_worker_count(),
        SizeOptions { include_links: opts.include_links },
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

fn has_bin_python_markers(bin_path: &Path) -> bool {
    let Ok(entries) = fs::read_dir(bin_path) else {
        return false;
    };
    let mut has_activate = false;
    let mut has_python = false;

    for entry in entries.filter_map(Result::ok) {
        let file_name = entry.file_name();
        let Some(file_name) = file_name.to_str() else {
            continue;
        };
        if file_name != "activate" && file_name != "python" {
            continue;
        }
        if !entry.file_type().is_ok_and(|file_type| file_type.is_file()) {
            continue;
        }

        if file_name == "activate" {
            has_activate = true;
        } else {
            has_python = true;
        }
        if has_activate && has_python {
            return true;
        }
    }

    false
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::io;
    use std::path::Path;
    use std::path::PathBuf;

    use repomop_core::{ArtifactKind, ScanOptions};
    use tempfile::TempDir;

    use super::{directory_entry_status, scan, scan_and_measure};

    #[cfg(unix)]
    fn symlink_dir(target: &Path, link: &Path) {
        std::os::unix::fs::symlink(target, link).unwrap();
    }

    fn make_opts(root: &Path) -> ScanOptions {
        ScanOptions {
            root_path: root.to_path_buf(),
            max_depth: None,
            include_links: false,
        }
    }

    #[test]
    fn scan_rejects_empty_root_path() {
        let err = scan(&ScanOptions {
            root_path: PathBuf::default(),
            max_depth: None,
            include_links: false,
        })
        .unwrap_err();

        assert_eq!(err, "root path is required");
    }

    #[test]
    fn scan_rejects_non_directory_root() {
        let temp = TempDir::new().unwrap();
        let file = temp.path().join("file.txt");
        fs::write(&file, "x").unwrap();

        let err = scan(&make_opts(&file)).unwrap_err();
        assert!(err.contains("root path is not a directory"));
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
        fs::write(temp.path().join("settings.gradle"), "rootProject.name='mono'")
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
    fn detects_virtualenvs_with_bin_layout() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("python");
        let venv = project.join("env");
        fs::create_dir_all(venv.join("bin")).unwrap();
        fs::write(project.join("requirements.txt"), "pytest\n").unwrap();
        fs::write(venv.join("bin/activate"), "").unwrap();
        fs::write(venv.join("bin/python"), "").unwrap();

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

    #[test]
    fn directory_entry_status_rejects_regular_file_when_links_disabled() {
        let temp = TempDir::new().unwrap();
        let file = temp.path().join("file.txt");
        fs::write(&file, "x").unwrap();
        let file_type = fs::symlink_metadata(&file).unwrap().file_type();

        assert_eq!(
            directory_entry_status(&file, false, &Ok(file_type)).unwrap(),
            (false, false)
        );
    }

    #[test]
    fn directory_entry_status_accepts_directory() {
        let temp = TempDir::new().unwrap();
        let dir = temp.path().join("dir");
        fs::create_dir(&dir).unwrap();
        let file_type = fs::symlink_metadata(&dir).unwrap().file_type();

        assert_eq!(
            directory_entry_status(&dir, false, &Ok(file_type)).unwrap(),
            (true, false)
        );
    }

    #[test]
    #[cfg(unix)]
    fn directory_entry_status_resolves_symlink_directories_when_requested() {
        let temp = TempDir::new().unwrap();
        let dir = temp.path().join("dir");
        let link = temp.path().join("dir-link");
        fs::create_dir(&dir).unwrap();
        symlink_dir(&dir, &link);
        let file_type = fs::symlink_metadata(&link).unwrap().file_type();

        assert_eq!(
            directory_entry_status(&link, true, &Ok(file_type)).unwrap(),
            (true, true)
        );
    }

    #[test]
    fn directory_entry_status_propagates_file_type_errors() {
        let err = directory_entry_status(
            Path::new("ignored"),
            false,
            &Err(io::Error::other("boom")),
        )
        .unwrap_err();

        assert_eq!(err.kind(), io::ErrorKind::Other);
        assert_eq!(err.to_string(), "boom");
    }
}
