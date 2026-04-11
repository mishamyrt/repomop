mod context;
mod detect;
mod matchers;
mod walk;

use repomop_core::{Artifact, ScanOptions};

pub fn scan(opts: &ScanOptions) -> Result<(Vec<Artifact>, Vec<String>), String> {
    walk::scan(opts)
}

pub fn scan_and_measure(
    opts: &ScanOptions,
) -> Result<(Vec<Artifact>, Vec<String>), String> {
    walk::scan_and_measure(opts)
}
