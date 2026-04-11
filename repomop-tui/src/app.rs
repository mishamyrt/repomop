use std::collections::BTreeSet;
use std::sync::mpsc::{self, Receiver, Sender, TryRecvError};
use std::thread;

use repomop_core::{Artifact, ScanOptions, sort_artifacts_by_size_desc};
use repomop_fs::{DeleteResult, delete_artifacts};
use repomop_scanner::scan_and_measure;

use crate::widgets::{DELETE_SPINNER_FRAME_COUNT, SCAN_SPINNER_FRAME_COUNT};

#[derive(Debug, Clone, Default)]
pub struct SessionResult {
    pub fatal_error: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum ViewState {
    Loading,
    List,
    Confirm,
    Deleting,
    Done,
    Error,
}

#[derive(Debug)]
pub(crate) enum BackgroundEvent {
    ScanFinished(Result<(Vec<Artifact>, Vec<String>), String>),
    DeleteFinished(DeleteResult),
}

#[derive(Debug)]
pub(crate) struct App {
    pub(crate) opts: ScanOptions,
    pub(crate) state: ViewState,
    pub(crate) artifacts: Vec<Artifact>,
    pub(crate) selected: BTreeSet<usize>,
    /// Pre-computed list of artifacts for the confirm screen.
    pub(crate) cached_selection: Option<Vec<Artifact>>,
    pub(crate) cursor: usize,
    pub(crate) confirm_offset: usize,
    pub(crate) scan_warnings: Vec<String>,
    pub(crate) delete_result: DeleteResult,
    pub(crate) message: String,
    pub(crate) fatal_error: Option<String>,
    pub(crate) scan_spinner_index: usize,
    pub(crate) delete_spinner_index: usize,
    pub(crate) should_quit: bool,
    /// Set when the scan found zero artifacts (avoids fragile string comparison).
    pub(crate) no_artifacts_found: bool,
    pub(crate) background_rx: Receiver<BackgroundEvent>,
    pub(crate) background_tx: Sender<BackgroundEvent>,
}

impl App {
    pub(crate) fn new(opts: ScanOptions) -> Self {
        let (background_tx, background_rx) = mpsc::channel();
        let app = Self {
            opts,
            state: ViewState::Loading,
            artifacts: Vec::new(),
            selected: BTreeSet::new(),
            cached_selection: None,
            cursor: 0,
            confirm_offset: 0,
            scan_warnings: Vec::new(),
            delete_result: DeleteResult::default(),
            message: String::new(),
            fatal_error: None,
            scan_spinner_index: 0,
            delete_spinner_index: 0,
            should_quit: false,
            no_artifacts_found: false,
            background_rx,
            background_tx,
        };
        app.spawn_scan();
        app
    }

    pub(crate) fn spawn_scan(&self) {
        let sender = self.background_tx.clone();
        let opts = self.opts.clone();
        thread::spawn(move || {
            let result = scan_and_measure(&opts);
            let _ = sender.send(BackgroundEvent::ScanFinished(result));
        });
    }

    pub(crate) fn spawn_delete(&self, artifacts: Vec<Artifact>) {
        let sender = self.background_tx.clone();
        thread::spawn(move || {
            let result = delete_artifacts(&artifacts);
            let _ = sender.send(BackgroundEvent::DeleteFinished(result));
        });
    }

    pub(crate) fn drain_background(&mut self) {
        loop {
            match self.background_rx.try_recv() {
                Ok(BackgroundEvent::ScanFinished(result)) => match result {
                    Ok((artifacts, warnings)) => {
                        self.artifacts = artifacts;
                        self.scan_warnings = warnings;
                        self.selected.clear();
                        self.cursor = 0;
                        if self.artifacts.is_empty() {
                            self.no_artifacts_found = true;
                            self.state = ViewState::Done;
                            self.message = "No artifacts found.".to_string();
                        } else {
                            self.state = ViewState::List;
                            if !self.scan_warnings.is_empty() {
                                self.message = "Some artifact sizes could not be fully calculated."
                                    .to_string();
                            }
                        }
                    }
                    Err(err) => {
                        self.state = ViewState::Error;
                        self.fatal_error = Some(err.clone());
                        self.message = err;
                    }
                },
                Ok(BackgroundEvent::DeleteFinished(result)) => {
                    self.delete_result = result;
                    self.state = ViewState::Done;
                    self.message = if self.delete_result.errors.is_empty() {
                        "Selected artifacts were removed.".to_string()
                    } else {
                        "Some artifacts could not be removed.".to_string()
                    };
                }
                Err(TryRecvError::Empty | TryRecvError::Disconnected) => break,
            }
        }
    }

    pub(crate) fn tick(&mut self) {
        match self.state {
            ViewState::Loading => {
                self.scan_spinner_index =
                    (self.scan_spinner_index + 1) % SCAN_SPINNER_FRAME_COUNT;
            }
            ViewState::Deleting => {
                self.delete_spinner_index =
                    (self.delete_spinner_index + 1) % DELETE_SPINNER_FRAME_COUNT;
            }
            ViewState::List
            | ViewState::Confirm
            | ViewState::Done
            | ViewState::Error => {}
        }
    }

    pub(crate) fn selected_count(&self) -> usize {
        self.selected.len()
    }

    pub(crate) fn selected_size(&self) -> u64 {
        self.selected
            .iter()
            .filter_map(|index| self.artifacts.get(*index))
            .map(|artifact| artifact.size_bytes)
            .sum()
    }

    pub(crate) fn selected_artifacts(&self) -> Vec<Artifact> {
        let mut artifacts: Vec<_> = self
            .selected
            .iter()
            .filter_map(|index| self.artifacts.get(*index).cloned())
            .collect();
        sort_artifacts_by_size_desc(&mut artifacts);
        artifacts
    }

    /// Returns a reference to the confirm-screen artifact list without cloning.
    pub(crate) fn confirm_artifacts(&self) -> &[Artifact] {
        self.cached_selection.as_deref().unwrap_or(&[])
    }
}
