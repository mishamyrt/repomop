use std::env;
use std::io::{self, Write};
use std::path::{Path, PathBuf};

use repomop_core::{
    Artifact, ScanOptions, format_bytes, normalize_path, relative_path_or_self,
};
use repomop_fs::{DeleteResult, delete_artifacts};
use repomop_scanner::scan_and_measure;

#[allow(clippy::struct_excessive_bools)]
#[derive(Debug, Clone, PartialEq, Eq)]
struct CliOptions {
    path: PathBuf,
    max_depth: Option<usize>,
    dry_run: bool,
    yes: bool,
    version: bool,
    include_links: bool,
}

enum ParseOutcome {
    Options(CliOptions),
    Help,
}

pub fn run(
    args: &[String],
    stdout: &mut impl Write,
    stderr: &mut impl Write,
) -> i32 {
    let opts = match parse_flags(args, stdout) {
        Ok(ParseOutcome::Options(opts)) => opts,
        Ok(ParseOutcome::Help) => return 0,
        Err(err) => {
            let _ = writeln!(stderr, "{err}");
            return 1;
        }
    };

    if opts.version {
        let _ = writeln!(stdout, "{}", env!("CARGO_PKG_VERSION"));
        return 0;
    }

    let root_path = match absolute_path(&opts.path) {
        Ok(path) => path,
        Err(err) => {
            let _ = writeln!(stderr, "resolve path: {err}");
            return 1;
        }
    };

    let scan_opts = ScanOptions {
        root_path: root_path.clone(),
        max_depth: opts.max_depth,
        include_links: opts.include_links,
    };

    if opts.dry_run || opts.yes {
        let (artifacts, warnings) = match scan_and_measure(&scan_opts) {
            Ok(result) => result,
            Err(err) => {
                let _ = writeln!(stderr, "scan failed: {err}");
                return 1;
            }
        };

        if opts.dry_run {
            print_dry_run(stdout, &root_path, &artifacts, &warnings).ok();
            return 0;
        }

        let result = delete_artifacts(&artifacts);
        print_delete_summary(stdout, &root_path, &artifacts, &warnings, &result)
            .ok();
        return i32::from(!result.errors.is_empty());
    }

    match repomop_tui::run(scan_opts) {
        Ok(result) => {
            if let Some(err) = result.fatal_error {
                let _ = writeln!(stderr, "scan failed: {err}");
                1
            } else {
                0
            }
        }
        Err(err) => {
            let _ = writeln!(stderr, "tui failed: {err}");
            1
        }
    }
}

fn parse_flags(
    args: &[String],
    stdout: &mut impl Write,
) -> Result<ParseOutcome, String> {
    let cwd =
        env::current_dir().map_err(|err| format!("get working directory: {err}"))?;
    let mut opts = CliOptions {
        path: cwd,
        max_depth: None,
        dry_run: false,
        yes: false,
        version: false,
        include_links: false,
    };

    let mut index = 0usize;
    while index < args.len() {
        let arg = &args[index];
        match arg.as_str() {
            "-h" | "--help" => {
                print_help(stdout).map_err(|err| err.to_string())?;
                return Ok(ParseOutcome::Help);
            }
            "--dry-run" => opts.dry_run = true,
            "--yes" => opts.yes = true,
            "--version" => opts.version = true,
            "--include-links" => opts.include_links = true,
            "--path" => {
                index += 1;
                let Some(value) = args.get(index) else {
                    return Err("missing value for --path".to_string());
                };
                opts.path = PathBuf::from(value);
            }
            "--max-depth" => {
                index += 1;
                let Some(value) = args.get(index) else {
                    return Err("missing value for --max-depth".to_string());
                };
                opts.max_depth = parse_max_depth(value)?;
            }
            _ if arg.starts_with("--path=") => {
                opts.path = PathBuf::from(arg.trim_start_matches("--path="));
            }
            _ if arg.starts_with("--max-depth=") => {
                let value = arg.trim_start_matches("--max-depth=");
                opts.max_depth = parse_max_depth(value)?;
            }
            _ => return Err(format!("unknown flag: {arg}")),
        }
        index += 1;
    }

    if opts.dry_run && opts.yes {
        return Err("--dry-run and --yes cannot be used together".to_string());
    }

    Ok(ParseOutcome::Options(opts))
}

fn parse_max_depth(value: &str) -> Result<Option<usize>, String> {
    let parsed: isize =
        value.parse().map_err(|_| "max-depth must be -1 or >= 0".to_string())?;
    match parsed {
        -1 => Ok(None),
        0.. => usize::try_from(parsed)
            .map(Some)
            .map_err(|_| "max-depth must be -1 or >= 0".to_string()),
        _ => Err("max-depth must be -1 or >= 0".to_string()),
    }
}

fn absolute_path(path: &Path) -> io::Result<PathBuf> {
    if path.is_absolute() {
        Ok(normalize_path(path))
    } else {
        Ok(normalize_path(&env::current_dir()?.join(path)))
    }
}

fn print_help(stdout: &mut impl Write) -> io::Result<()> {
    writeln!(
        stdout,
        "Usage: repomop [--path PATH] [--max-depth N] [--dry-run | --yes] [--include-links] [--version]"
    )?;
    writeln!(
        stdout,
        "  --path           Root directory to scan (defaults to the current directory)"
    )?;
    writeln!(
        stdout,
        "  --max-depth      Maximum traversal depth (-1 means unlimited)"
    )?;
    writeln!(stdout, "  --dry-run        List artifacts without deleting")?;
    writeln!(
        stdout,
        "  --yes            Delete all found artifacts without interactive confirmation"
    )?;
    writeln!(
        stdout,
        "  --include-links  Follow symlinked directories and count hard-linked files"
    )?;
    writeln!(stdout, "  --version        Print version and exit")?;
    Ok(())
}

fn print_dry_run(
    stdout: &mut impl Write,
    root: &Path,
    artifacts: &[Artifact],
    warnings: &[String],
) -> io::Result<()> {
    writeln!(stdout, "repomop dry-run")?;
    if artifacts.is_empty() {
        writeln!(stdout, "No artifacts found.")?;
        return Ok(());
    }

    let mut total = 0u64;
    for artifact in artifacts {
        total = total.saturating_add(artifact.size_bytes);
        writeln!(
            stdout,
            "- {:>8}  {}  {}",
            format_bytes(artifact.size_bytes),
            relative_path_or_self(root, &artifact.path).display(),
            artifact.kind
        )?;
    }
    writeln!(
        stdout,
        "Found: {} artifacts, Potential free space: {}",
        artifacts.len(),
        format_bytes(total)
    )?;
    if !warnings.is_empty() {
        writeln!(stdout, "Warnings: {} size calculation warnings", warnings.len())?;
    }
    Ok(())
}

fn print_delete_summary(
    stdout: &mut impl Write,
    root: &Path,
    artifacts: &[Artifact],
    warnings: &[String],
    result: &DeleteResult,
) -> io::Result<()> {
    writeln!(stdout, "repomop --yes")?;
    if artifacts.is_empty() {
        writeln!(stdout, "No artifacts found.")?;
        return Ok(());
    }

    writeln!(stdout, "Found artifacts: {}", artifacts.len())?;
    writeln!(stdout, "Deleted: {}", result.deleted.len())?;
    writeln!(stdout, "Freed space: {}", format_bytes(result.freed_bytes))?;
    if !result.errors.is_empty() {
        writeln!(stdout, "Delete errors: {}", result.errors.len())?;
        for item in &result.errors {
            writeln!(
                stdout,
                "- {}: {}",
                relative_path_or_self(root, &item.artifact.path).display(),
                item.error
            )?;
        }
    }
    if !warnings.is_empty() {
        writeln!(stdout, "Warnings: {} size calculation warnings", warnings.len())?;
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::run;

    #[test]
    fn dry_run_does_not_delete() {
        let temp = TempDir::new().unwrap();
        let project = temp.path().join("project");
        fs::create_dir_all(project.join("node_modules")).unwrap();
        fs::write(project.join("package.json"), "{}").unwrap();
        fs::write(project.join("node_modules/a.js"), "console.log('x')").unwrap();

        let args = vec![
            "--path".to_string(),
            temp.path().display().to_string(),
            "--dry-run".to_string(),
        ];
        let mut stdout = Vec::new();
        let mut stderr = Vec::new();
        let exit_code = run(&args, &mut stdout, &mut stderr);

        assert_eq!(exit_code, 0);
        assert!(String::from_utf8(stdout).unwrap().contains("repomop dry-run"));
        assert!(project.join("node_modules").exists());
    }

    #[test]
    fn yes_deletes_all_artifacts() {
        let temp = TempDir::new().unwrap();
        let js_project = temp.path().join("js");
        let py_project = temp.path().join("py");
        let venv = py_project.join("my_env");

        fs::create_dir_all(js_project.join("node_modules")).unwrap();
        fs::write(js_project.join("package.json"), "{}").unwrap();
        fs::write(js_project.join("node_modules/a.js"), "console.log('x')").unwrap();
        fs::create_dir_all(&venv).unwrap();
        fs::write(venv.join("pyvenv.cfg"), "home=/usr/bin").unwrap();

        let args = vec![
            "--path".to_string(),
            temp.path().display().to_string(),
            "--yes".to_string(),
        ];
        let mut stdout = Vec::new();
        let mut stderr = Vec::new();
        let exit_code = run(&args, &mut stdout, &mut stderr);

        assert_eq!(exit_code, 0);
        assert!(String::from_utf8(stdout).unwrap().contains("Deleted: 2"));
        assert!(!js_project.join("node_modules").exists());
        assert!(!venv.exists());
    }

    #[test]
    fn rejects_conflicting_flags() {
        let args = vec!["--dry-run".to_string(), "--yes".to_string()];
        let mut stdout = Vec::new();
        let mut stderr = Vec::new();
        let exit_code = run(&args, &mut stdout, &mut stderr);

        assert_eq!(exit_code, 1);
        assert!(
            String::from_utf8(stderr).unwrap().contains("cannot be used together")
        );
    }
}
