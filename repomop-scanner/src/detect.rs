use std::path::{Path, PathBuf};

use repomop_core::{
    ARTIFACT_DEFINITIONS, Artifact, ArtifactDefinition, ArtifactKind,
    ArtifactMatcher, normalize_path,
};

use crate::context::ScanContext;
use crate::matchers::{has_path_suffix, is_within_root, path_distance};

#[derive(Debug, Clone)]
struct Candidate {
    kind: ArtifactKind,
    project_root: PathBuf,
    distance: usize,
}

pub(crate) fn detect_artifact(
    ctx: &mut ScanContext,
    path: &Path,
) -> Option<Artifact> {
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

pub(crate) fn detect_virtual_env_artifact(
    ctx: &mut ScanContext,
    path: &Path,
) -> Option<Artifact> {
    let definition = ARTIFACT_DEFINITIONS.iter().find(|definition| {
        matches!(definition.matcher, ArtifactMatcher::VirtualEnv)
    })?;

    Some(Artifact {
        kind: definition.kind,
        path: path.to_path_buf(),
        project_root: infer_venv_project_root(ctx, path, definition.project_markers),
        size_bytes: 0,
    })
}

fn detect_candidate(
    ctx: &mut ScanContext,
    definition: &ArtifactDefinition,
    path: &Path,
) -> Option<Candidate> {
    if !definition_matches(definition, path) {
        return None;
    }

    let project_root = match definition.matcher {
        ArtifactMatcher::VirtualEnv => return None,
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

fn definition_matches(definition: &ArtifactDefinition, path: &Path) -> bool {
    let Some(base) = path.file_name().and_then(|name| name.to_str()) else {
        return false;
    };

    match definition.matcher {
        ArtifactMatcher::NamedDir(name) => base == name,
        ArtifactMatcher::PrefixDir(prefix) => {
            base.starts_with(prefix) && base.len() > prefix.len()
        }
        ArtifactMatcher::PathSuffix(parts) => has_path_suffix(path, parts),
        ArtifactMatcher::VirtualEnv => false,
    }
}

fn update_best(best: &mut Option<Candidate>, candidate: Candidate) {
    match best {
        Some(current) if current.distance <= candidate.distance => {}
        _ => *best = Some(candidate),
    }
}

fn infer_venv_project_root(
    ctx: &mut ScanContext,
    path: &Path,
    project_markers: &[&str],
) -> PathBuf {
    find_nearest_ancestor_with_marker(
        ctx,
        path.parent().unwrap_or(path),
        project_markers,
    )
    .unwrap_or_else(|| path.parent().unwrap_or(path).to_path_buf())
}

fn find_nearest_ancestor_with_marker(
    ctx: &mut ScanContext,
    start: &Path,
    markers: &[&str],
) -> Option<PathBuf> {
    let mut current = normalize_path(start);

    loop {
        if !is_within_root(&current, ctx.root()) {
            return None;
        }

        if markers.iter().any(|marker| ctx.has_marker(&current, marker)) {
            return Some(current);
        }

        if current == ctx.root() {
            return None;
        }
        let parent = current.parent()?;
        current = parent.to_path_buf();
    }
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::Path;

    use repomop_core::{ArtifactDefinition, ArtifactKind, ArtifactMatcher};
    use tempfile::TempDir;

    use super::{
        Candidate, definition_matches, detect_artifact, detect_candidate,
        detect_virtual_env_artifact, find_nearest_ancestor_with_marker,
        infer_venv_project_root, update_best,
    };
    use crate::context::ScanContext;

    fn definition(
        kind: ArtifactKind,
        matcher: ArtifactMatcher,
        project_markers: &'static [&'static str],
    ) -> ArtifactDefinition {
        ArtifactDefinition { kind, matcher, project_markers }
    }

    #[test]
    fn definition_matches_supports_all_matcher_kinds() {
        let temp = TempDir::new().unwrap();
        let root = temp.path();
        let named = root.join("node_modules");
        let prefixed = root.join("cmake-build-debug");
        let suffixed = root.join("vendor/bundle");

        fs::create_dir_all(&named).unwrap();
        fs::create_dir_all(&prefixed).unwrap();
        fs::create_dir_all(&suffixed).unwrap();

        assert!(definition_matches(
            &definition(
                ArtifactKind::NodeModules,
                ArtifactMatcher::NamedDir("node_modules"),
                &["package.json"],
            ),
            &named,
        ));
        assert!(definition_matches(
            &definition(
                ArtifactKind::Cmake,
                ArtifactMatcher::PrefixDir("cmake-build-"),
                &["CMakeLists.txt"],
            ),
            &prefixed,
        ));
        assert!(definition_matches(
            &definition(
                ArtifactKind::Ruby,
                ArtifactMatcher::PathSuffix(&["vendor", "bundle"]),
                &["Gemfile"],
            ),
            &suffixed,
        ));
    }

    #[test]
    fn detect_candidate_returns_none_when_project_markers_are_missing() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        let artifact = project.join("node_modules");
        fs::create_dir_all(&artifact).unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        let candidate = detect_candidate(
            &mut ctx,
            &definition(
                ArtifactKind::NodeModules,
                ArtifactMatcher::NamedDir("node_modules"),
                &["package.json"],
            ),
            &artifact,
        );

        assert!(candidate.is_none());
    }

    #[test]
    fn detect_artifact_prefers_nearest_matching_project_root() {
        let temp = TempDir::new().unwrap();
        let workspace = temp.path().join("workspace");
        let app = workspace.join("apps/mobile");
        let build = app.join("build");
        fs::create_dir_all(&build).unwrap();
        fs::write(workspace.join("settings.gradle"), "rootProject.name='mono'")
            .unwrap();
        fs::write(app.join("pubspec.yaml"), "name: mobile").unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        let artifact = detect_artifact(&mut ctx, &build).unwrap();

        assert_eq!(artifact.kind, ArtifactKind::DartFlutter);
        assert_eq!(artifact.project_root, app);
    }

    #[test]
    fn update_best_keeps_smallest_distance() {
        let mut best = Some(Candidate {
            kind: ArtifactKind::JavaGradle,
            project_root: Path::new("/tmp/project").to_path_buf(),
            distance: 2,
        });

        update_best(
            &mut best,
            Candidate {
                kind: ArtifactKind::DartFlutter,
                project_root: Path::new("/tmp/project/app").to_path_buf(),
                distance: 1,
            },
        );

        assert_eq!(best.unwrap().kind, ArtifactKind::DartFlutter);
    }

    #[test]
    fn detect_virtual_env_artifact_uses_python_project_root() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        let venv = project.join(".venv");
        fs::create_dir_all(&venv).unwrap();
        fs::write(project.join("pyproject.toml"), "[project]").unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        let artifact = detect_virtual_env_artifact(&mut ctx, &venv).unwrap();

        assert_eq!(artifact.kind, ArtifactKind::PythonVenv);
        assert_eq!(artifact.project_root, project);
    }

    #[test]
    fn detect_artifact_does_not_probe_virtualenv_internals() {
        let temp = TempDir::new().unwrap();
        let venv = temp.path().join(".venv");
        fs::create_dir_all(venv.join("bin")).unwrap();
        fs::write(venv.join("bin/activate"), "").unwrap();
        fs::write(venv.join("bin/python"), "").unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        assert!(detect_artifact(&mut ctx, &venv).is_none());
    }

    #[test]
    fn infer_venv_project_root_falls_back_to_parent_when_no_markers_exist() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("python");
        let venv = project.join(".venv");
        fs::create_dir_all(&venv).unwrap();

        let mut ctx = ScanContext::new(temp.path().to_path_buf());
        let project_root =
            infer_venv_project_root(&mut ctx, &venv, &["pyproject.toml"]);

        assert_eq!(project_root, project);
    }

    #[test]
    fn find_nearest_ancestor_with_marker_stops_at_scan_root() {
        let temp = TempDir::new().unwrap();
        let scan_root = temp.path().join("scan");
        let nested = scan_root.join("a/b");
        fs::create_dir_all(&nested).unwrap();
        fs::write(temp.path().join("package.json"), "{}").unwrap();

        let mut ctx = ScanContext::new(scan_root);
        let project_root =
            find_nearest_ancestor_with_marker(&mut ctx, &nested, &["package.json"]);

        assert!(project_root.is_none());
    }
}
