use std::fs;
use std::io;
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::mpsc;
use std::thread;

use repomop_core::Artifact;

#[derive(Debug)]
pub struct DeleteError {
    pub artifact: Artifact,
    pub error: io::Error,
}

#[derive(Debug, Default)]
pub struct DeleteResult {
    pub deleted: Vec<Artifact>,
    pub errors: Vec<DeleteError>,
    pub freed_bytes: u64,
}

#[derive(Debug)]
enum DeletionOutcome {
    Deleted { artifact: Artifact },
    Missing { artifact: Artifact },
    Failed { artifact: Artifact, error: io::Error },
}

pub fn delete_artifacts(artifacts: &[Artifact]) -> DeleteResult {
    if artifacts.is_empty() {
        return DeleteResult::default();
    }

    let worker_count = worker_count_for(artifacts.len());
    let next_index = AtomicUsize::new(0);
    let (sender, receiver) = mpsc::channel::<DeletionOutcome>();

    thread::scope(|scope| {
        for _ in 0..worker_count {
            let sender = sender.clone();
            let next_index = &next_index;
            scope.spawn(move || {
                loop {
                    let idx = next_index.fetch_add(1, Ordering::Relaxed);
                    let Some(artifact) = artifacts.get(idx) else {
                        break;
                    };
                    let outcome = match fs::remove_dir_all(&artifact.path) {
                        Ok(()) => {
                            DeletionOutcome::Deleted { artifact: artifact.clone() }
                        }
                        Err(err) if err.kind() == io::ErrorKind::NotFound => {
                            DeletionOutcome::Missing { artifact: artifact.clone() }
                        }
                        Err(error) => DeletionOutcome::Failed {
                            artifact: artifact.clone(),
                            error,
                        },
                    };
                    if sender.send(outcome).is_err() {
                        break;
                    }
                }
            });
        }
        drop(sender);

        let mut result = DeleteResult {
            deleted: Vec::with_capacity(artifacts.len()),
            errors: Vec::new(),
            freed_bytes: 0,
        };

        for outcome in receiver {
            match outcome {
                DeletionOutcome::Deleted { artifact }
                | DeletionOutcome::Missing { artifact } => {
                    result.freed_bytes =
                        result.freed_bytes.saturating_add(artifact.size_bytes);
                    result.deleted.push(artifact);
                }
                DeletionOutcome::Failed { artifact, error } => {
                    result.errors.push(DeleteError { artifact, error });
                }
            }
        }

        result
    })
}

fn worker_count_for(n: usize) -> usize {
    thread::available_parallelism()
        .map(std::num::NonZero::get)
        .unwrap_or(1)
        .clamp(1, 8)
        .min(n)
}

/// Returns the filesystem path that failed to delete.
impl DeleteError {
    pub fn path(&self) -> &PathBuf {
        &self.artifact.path
    }
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
