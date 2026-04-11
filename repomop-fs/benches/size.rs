use std::fs;
use std::path::{Path, PathBuf};
use std::time::Duration;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use repomop_fs::{SizeOptions, directories};
use tempfile::TempDir;

struct SizeFixture {
    _temp: TempDir,
    roots: Vec<PathBuf>,
}

impl SizeFixture {
    fn artifact_roots() -> Self {
        let temp = TempDir::new().unwrap();
        let root = temp.path().join("workspace");
        fs::create_dir_all(&root).unwrap();

        let mut roots = Vec::new();
        for index in 0..24 {
            let target = root.join(format!("crates/tool-{index}/target"));
            for entry in 0..10 {
                write_blob(
                    &target.join(format!("debug/deps/lib-{entry}.rlib")),
                    1_536,
                );
                write_blob(
                    &target.join(format!("debug/incremental/cache-{entry}.bin")),
                    896,
                );
            }
            roots.push(target);
        }

        Self { _temp: temp, roots }
    }
}

fn write_blob(path: &Path, size: usize) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(path, vec![b'x'; size]).unwrap();
}

fn bench_size(c: &mut Criterion) {
    let fixture = Box::leak(Box::new(SizeFixture::artifact_roots()));

    let mut group = c.benchmark_group("repomop_fs");
    group.sample_size(20);
    group.warm_up_time(Duration::from_secs(1));
    group.measurement_time(Duration::from_secs(6));

    group.bench_function("directories_artifact_batch", |b| {
        b.iter(|| {
            let (sizes, warnings) = directories(
                black_box(fixture.roots.as_slice()),
                1,
                SizeOptions::default(),
            );
            black_box((sizes.len(), warnings.len()));
        });
    });

    group.finish();
}

criterion_group!(benches, bench_size);
criterion_main!(benches);
