use std::collections::{HashMap, HashSet, VecDeque};
#[cfg(target_os = "macos")]
use std::ffi::CString;
use std::fs::{self, Metadata};
#[cfg(target_os = "macos")]
use std::io;
#[cfg(target_os = "macos")]
use std::os::unix::ffi::OsStrExt;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, mpsc};
use std::thread;

#[cfg(unix)]
use std::os::unix::fs::MetadataExt;

#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct SizeOptions {
    pub include_links: bool,
}

#[derive(Debug)]
struct JobResult {
    path: PathBuf,
    size: u64,
    warnings: Vec<String>,
}

pub fn recommended_worker_count() -> usize {
    std::thread::available_parallelism()
        .map(std::num::NonZero::get)
        .unwrap_or(1)
        .clamp(1, 8)
}

pub fn directories(
    paths: &[PathBuf],
    workers: usize,
    opts: SizeOptions,
) -> (HashMap<PathBuf, u64>, Vec<String>) {
    let mut sizes = HashMap::with_capacity(paths.len());
    if paths.is_empty() {
        return (sizes, Vec::new());
    }

    let worker_count = workers.max(1).min(paths.len());
    let queue = Arc::new(Mutex::new(VecDeque::from(paths.to_vec())));
    let (sender, receiver) = mpsc::channel::<JobResult>();

    thread::scope(|scope| {
        for _ in 0..worker_count {
            let queue = Arc::clone(&queue);
            let sender = sender.clone();
            scope.spawn(move || {
                loop {
                    let next = {
                        let mut jobs = queue.lock().expect("queue poisoned");
                        jobs.pop_front()
                    };

                    let Some(path) = next else {
                        break;
                    };

                    let (size, warnings) = directory_size(&path, opts);
                    if sender.send(JobResult { path, size, warnings }).is_err() {
                        break;
                    }
                }
            });
        }
        drop(sender);

        let mut warnings = Vec::new();
        for result in receiver {
            sizes.insert(result.path, result.size);
            warnings.extend(result.warnings);
        }

        (sizes, warnings)
    })
}

fn directory_size(path: &Path, opts: SizeOptions) -> (u64, Vec<String>) {
    let mut walker = SizeWalker {
        opts,
        total: 0,
        warnings: Vec::new(),
        visited_dirs: HashSet::new(),
    };
    walker.walk(path);
    (walker.total, walker.warnings)
}

#[derive(Debug)]
struct SizeWalker {
    opts: SizeOptions,
    total: u64,
    warnings: Vec<String>,
    visited_dirs: HashSet<PathBuf>,
}

impl SizeWalker {
    fn walk(&mut self, path: &Path) {
        let metadata = match fs::symlink_metadata(path) {
            Ok(metadata) => metadata,
            Err(err) => {
                self.warn(path, &err);
                return;
            }
        };

        let file_type = metadata.file_type();
        if file_type.is_symlink() {
            if !self.opts.include_links {
                return;
            }

            match fs::metadata(path) {
                Ok(target) if target.is_dir() => self.walk_directory(path),
                Ok(target) if target.is_file() => {
                    self.total = self.total.saturating_add(reclaimable_file_size(
                        path,
                        &target,
                        self.opts.include_links,
                    ));
                }
                Ok(_) => {}
                Err(err) => self.warn(path, &err),
            }
            return;
        }

        if metadata.is_dir() {
            self.walk_directory(path);
            return;
        }

        if metadata.is_file() {
            self.total = self.total.saturating_add(reclaimable_file_size(
                path,
                &metadata,
                self.opts.include_links,
            ));
        }
    }

    fn walk_directory(&mut self, path: &Path) {
        if self.opts.include_links {
            match fs::canonicalize(path) {
                Ok(real_path) => {
                    if !self.visited_dirs.insert(real_path) {
                        return;
                    }
                }
                Err(err) => {
                    self.warn(path, &err);
                    return;
                }
            }
        }

        let entries = match fs::read_dir(path) {
            Ok(entries) => entries,
            Err(err) => {
                self.warn(path, &err);
                return;
            }
        };

        for entry in entries {
            match entry {
                Ok(entry) => self.walk(&entry.path()),
                Err(err) => self.warn(path, &err),
            }
        }
    }

    fn warn(&mut self, path: &Path, err: &std::io::Error) {
        self.warnings.push(format!("{}: {err}", path.display()));
    }
}

#[cfg(all(unix, not(target_os = "macos")))]
fn reclaimable_file_size(
    _path: &Path,
    metadata: &Metadata,
    include_links: bool,
) -> u64 {
    if !include_links && metadata.nlink() > 1 { 0 } else { metadata.len() }
}

#[cfg(target_os = "macos")]
fn reclaimable_file_size(
    path: &Path,
    metadata: &Metadata,
    include_links: bool,
) -> u64 {
    if !include_links && metadata.nlink() > 1 {
        return 0;
    }

    match private_data_size(path) {
        Ok(private_size) => private_size.min(metadata.len()),
        Err(_) => metadata.len(),
    }
}

#[cfg(target_os = "macos")]
fn private_data_size(path: &Path) -> io::Result<u64> {
    const ATTR_CMNEXT_PRIVATESIZE: u32 = 0x0000_0008;
    const PRIVATE_SIZE_BUF_LEN: usize = 12;

    let c_path = CString::new(path.as_os_str().as_bytes()).map_err(|_| {
        io::Error::new(io::ErrorKind::InvalidInput, "path contains NUL")
    })?;
    let mut attr_list = libc::attrlist {
        bitmapcount: 5,
        reserved: 0,
        commonattr: 0,
        volattr: 0,
        dirattr: 0,
        fileattr: 0,
        forkattr: ATTR_CMNEXT_PRIVATESIZE,
    };
    let mut buffer = [0u8; PRIVATE_SIZE_BUF_LEN];

    let result = unsafe {
        libc::getattrlist(
            c_path.as_ptr(),
            (&raw mut attr_list).cast(),
            buffer.as_mut_ptr().cast(),
            buffer.len(),
            libc::FSOPT_ATTR_CMN_EXTENDED,
        )
    };
    if result != 0 {
        return Err(io::Error::last_os_error());
    }

    let length = u32::from_le_bytes(buffer[..4].try_into().unwrap()) as usize;
    if length < PRIVATE_SIZE_BUF_LEN {
        return Err(io::Error::from_raw_os_error(libc::ENOTSUP));
    }

    Ok(u64::from_le_bytes(buffer[4..12].try_into().unwrap()))
}

#[cfg(not(unix))]
fn reclaimable_file_size(
    _path: &Path,
    metadata: &Metadata,
    _include_links: bool,
) -> u64 {
    metadata.len()
}

#[cfg(test)]
mod tests {
    use std::fs;
    use std::path::PathBuf;

    use tempfile::TempDir;

    use super::{SizeOptions, directories, recommended_worker_count};

    #[test]
    fn calculates_nested_sizes() {
        let temp = TempDir::new().unwrap();
        let dir_a = temp.path().join("a");
        let dir_b = temp.path().join("b");

        fs::create_dir_all(dir_a.join("nested")).unwrap();
        fs::create_dir_all(&dir_b).unwrap();
        fs::write(dir_a.join("nested").join("f1.bin"), [0u8; 10]).unwrap();
        fs::write(dir_a.join("nested").join("f2.bin"), [0u8; 15]).unwrap();
        fs::write(dir_b.join("f3.bin"), [0u8; 8]).unwrap();

        let (sizes, warnings) =
            directories(&[dir_a.clone(), dir_b.clone()], 2, SizeOptions::default());
        assert!(warnings.is_empty());
        assert_eq!(sizes[&dir_a], 25);
        assert_eq!(sizes[&dir_b], 8);
    }

    #[test]
    fn clamps_worker_count() {
        let workers = recommended_worker_count();
        assert!((1..=8).contains(&workers));
    }

    #[test]
    #[cfg(unix)]
    fn excludes_hard_linked_files_by_default() {
        let temp = TempDir::new().unwrap();
        let dir = temp.path().join("node_modules");
        let store = temp.path().join("store");
        fs::create_dir_all(&dir).unwrap();
        fs::create_dir_all(&store).unwrap();

        fs::write(dir.join("unique.bin"), [0u8; 100]).unwrap();
        let store_path = store.join("shared.bin");
        fs::write(&store_path, [0u8; 200]).unwrap();
        fs::hard_link(&store_path, dir.join("shared.bin")).unwrap();

        let (sizes, warnings) =
            directories(std::slice::from_ref(&dir), 1, SizeOptions::default());
        assert!(warnings.is_empty());
        assert_eq!(sizes[&dir], 100);
    }

    #[test]
    #[cfg(unix)]
    fn includes_hard_links_when_requested() {
        let temp = TempDir::new().unwrap();
        let dir = temp.path().join("node_modules");
        let store = temp.path().join("store");
        fs::create_dir_all(&dir).unwrap();
        fs::create_dir_all(&store).unwrap();

        fs::write(dir.join("unique.bin"), [0u8; 100]).unwrap();
        let store_path = store.join("shared.bin");
        fs::write(&store_path, [0u8; 200]).unwrap();
        fs::hard_link(&store_path, dir.join("shared.bin")).unwrap();

        let (sizes, warnings) = directories(
            std::slice::from_ref(&dir),
            1,
            SizeOptions { include_links: true },
        );
        assert!(warnings.is_empty());
        assert_eq!(sizes[&dir], 300);
    }

    #[test]
    #[cfg(unix)]
    fn skips_symlinks_by_default() {
        let temp = TempDir::new().unwrap();
        let dir = temp.path().join("project");
        let external = temp.path().join("external");
        fs::create_dir_all(&dir).unwrap();
        fs::create_dir_all(&external).unwrap();
        fs::write(external.join("big.bin"), [0u8; 100]).unwrap();
        std::os::unix::fs::symlink(&external, dir.join("link")).unwrap();
        fs::write(dir.join("real.bin"), [0u8; 50]).unwrap();

        let (sizes, _) =
            directories(std::slice::from_ref(&dir), 1, SizeOptions::default());
        assert_eq!(sizes[&dir], 50);
    }

    #[test]
    #[cfg(unix)]
    fn follows_symlinked_roots_when_requested() {
        let temp = TempDir::new().unwrap();
        let target = temp.path().join("target");
        let link = temp.path().join("node_modules");
        fs::create_dir_all(&target).unwrap();
        fs::write(target.join("dep.bin"), [0u8; 90]).unwrap();
        std::os::unix::fs::symlink(&target, &link).unwrap();

        let (sizes, warnings) = directories(
            std::slice::from_ref(&link),
            1,
            SizeOptions { include_links: true },
        );
        assert!(warnings.is_empty());
        assert_eq!(sizes[&link], 90);
    }

    #[test]
    fn nonexistent_paths_return_zero_size() {
        let missing = PathBuf::from("/nonexistent/path/xyz");
        let (sizes, _) =
            directories(std::slice::from_ref(&missing), 1, SizeOptions::default());
        assert_eq!(sizes[&missing], 0);
    }
}
