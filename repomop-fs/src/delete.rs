use std::fs;

use repomop_core::Artifact;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeleteError {
    pub artifact: Artifact,
    pub error: String,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct DeleteResult {
    pub deleted: Vec<Artifact>,
    pub errors: Vec<DeleteError>,
    pub freed_bytes: u64,
}

pub fn delete_artifacts(artifacts: &[Artifact]) -> DeleteResult {
    let mut result = DeleteResult {
        deleted: Vec::with_capacity(artifacts.len()),
        errors: Vec::new(),
        freed_bytes: 0,
    };

    for artifact in artifacts {
        match fs::remove_dir_all(&artifact.path) {
            Ok(()) => {
                result.freed_bytes =
                    result.freed_bytes.saturating_add(artifact.size_bytes);
                result.deleted.push(artifact.clone());
            }
            Err(err) if err.kind() == std::io::ErrorKind::NotFound => {
                result.freed_bytes =
                    result.freed_bytes.saturating_add(artifact.size_bytes);
                result.deleted.push(artifact.clone());
            }
            Err(err) => result.errors.push(DeleteError {
                artifact: artifact.clone(),
                error: err.to_string(),
            }),
        }
    }

    result
}

#[cfg(test)]
mod tests {
    use std::fs;

    use repomop_core::{Artifact, ArtifactKind};
    use tempfile::TempDir;

    use super::delete_artifacts;

    #[test]
    fn deletes_selected_directories() {
        let temp = TempDir::new().unwrap();
        let one = temp.path().join("one");
        let two = temp.path().join("two");

        fs::create_dir_all(&one).unwrap();
        fs::create_dir_all(&two).unwrap();
        fs::write(one.join("a.bin"), [0u8; 10]).unwrap();
        fs::write(two.join("b.bin"), [0u8; 20]).unwrap();

        let artifacts = vec![Artifact {
            kind: ArtifactKind::NodeModules,
            path: one.clone(),
            project_root: temp.path().to_path_buf(),
            size_bytes: 10,
        }];

        let result = delete_artifacts(&artifacts);
        assert_eq!(result.deleted.len(), 1);
        assert_eq!(result.errors.len(), 0);
        assert_eq!(result.freed_bytes, 10);
        assert!(!one.exists());
        assert!(two.exists());
    }

    #[test]
    fn nonexistent_paths_are_treated_as_deleted() {
        let artifact = Artifact {
            kind: ArtifactKind::NodeModules,
            path: TempDir::new().unwrap().path().join("missing"),
            project_root: std::path::PathBuf::new(),
            size_bytes: 42,
        };

        let result = delete_artifacts(&[artifact]);
        assert_eq!(result.deleted.len(), 1);
        assert!(result.errors.is_empty());
        assert_eq!(result.freed_bytes, 42);
    }
}
