use std::fs;
use std::path::Path;
use std::time::Duration;

use criterion::{Criterion, black_box, criterion_group, criterion_main};
use repomop_core::ScanOptions;
use tempfile::TempDir;

struct ScannerFixture {
    _temp: TempDir,
    opts: ScanOptions,
}

impl ScannerFixture {
    fn mixed_workspace() -> Self {
        let temp = TempDir::new().unwrap();
        let root = temp.path().join("workspace");
        fs::create_dir_all(&root).unwrap();

        for index in 0..18 {
            let js_project = root.join(format!("apps/web-{index}"));
            write_text(&js_project.join("package.json"), "{}\n");
            for dep in 0..8 {
                write_blob(
                    &js_project.join(format!("node_modules/pkg-{dep}/index.js")),
                    1_024,
                );
            }

            let rust_project = root.join(format!("crates/tool-{index}"));
            write_text(
                &rust_project.join("Cargo.toml"),
                "[package]\nname = \"tool\"\nversion = \"0.1.0\"\n",
            );
            for artifact in 0..6 {
                write_blob(
                    &rust_project
                        .join(format!("target/debug/deps/lib{artifact}.rlib")),
                    2_048,
                );
            }

            let flutter_project = root.join(format!("mobile/app-{index}"));
            write_text(&flutter_project.join("pubspec.yaml"), "name: app\n");
            for blob in 0..5 {
                write_blob(
                    &flutter_project.join(format!("build/cache/blob-{blob}.bin")),
                    1_536,
                );
            }

            let gradle_project = root.join(format!("services/jvm-{index}"));
            write_text(
                &gradle_project.join("build.gradle.kts"),
                "plugins { java }\n",
            );
            for jar in 0..4 {
                write_blob(
                    &gradle_project.join(format!("build/libs/lib-{jar}.jar")),
                    1_280,
                );
            }

            fs::create_dir_all(root.join(format!("docs/guide-{index}/nested/a/b")))
                .unwrap();
        }

        Self {
            opts: ScanOptions {
                root_path: root,
                max_depth: None,
                include_links: false,
            },
            _temp: temp,
        }
    }

    fn wildcard_workspace() -> Self {
        let temp = TempDir::new().unwrap();
        let root = temp.path().join("workspace");
        fs::create_dir_all(&root).unwrap();

        for index in 0..24 {
            let haskell_project = root.join(format!("haskell/pkg-{index}"));
            for entry in 0..18 {
                write_text(
                    &haskell_project.join(format!("notes-{entry}.cabal.backup")),
                    "ignored\n",
                );
            }
            write_text(
                &haskell_project.join(format!("package-{index}.cabal")),
                "name: demo\n",
            );
            write_blob(&haskell_project.join(".stack-work/dist/build.bin"), 768);

            let terraform_project = root.join(format!("infra/env-{index}"));
            for entry in 0..18 {
                write_text(
                    &terraform_project.join(format!("vars-{entry}.tf.bak")),
                    "ignored\n",
                );
            }
            write_text(
                &terraform_project.join(format!("service-{index}.tf")),
                "terraform {}\n",
            );
            write_blob(&terraform_project.join(".terraform/plugins/cache.bin"), 768);
        }

        Self {
            opts: ScanOptions {
                root_path: root,
                max_depth: None,
                include_links: false,
            },
            _temp: temp,
        }
    }
}

fn write_blob(path: &Path, size: usize) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(path, vec![b'x'; size]).unwrap();
}

fn write_text(path: &Path, contents: &str) {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).unwrap();
    }
    fs::write(path, contents).unwrap();
}

fn bench_scan(c: &mut Criterion) {
    let mixed = Box::leak(Box::new(ScannerFixture::mixed_workspace()));
    let wildcard = Box::leak(Box::new(ScannerFixture::wildcard_workspace()));

    let mut group = c.benchmark_group("repomop_scanner");
    group.sample_size(20);
    group.warm_up_time(Duration::from_secs(1));
    group.measurement_time(Duration::from_secs(6));

    group.bench_function("scan_mixed_workspace", |b| {
        b.iter(|| {
            let (artifacts, warnings) =
                repomop_scanner::scan(black_box(&mixed.opts)).unwrap();
            black_box((artifacts.len(), warnings.len()));
        });
    });

    group.bench_function("scan_and_measure_mixed_workspace", |b| {
        b.iter(|| {
            let (artifacts, warnings) =
                repomop_scanner::scan_and_measure(black_box(&mixed.opts)).unwrap();
            black_box((artifacts.len(), warnings.len()));
        });
    });

    group.bench_function("scan_wildcard_marker_workspace", |b| {
        b.iter(|| {
            let (artifacts, warnings) =
                repomop_scanner::scan(black_box(&wildcard.opts)).unwrap();
            black_box((artifacts.len(), warnings.len()));
        });
    });

    group.finish();
}

criterion_group!(benches, bench_scan);
criterion_main!(benches);
