use std::collections::BTreeSet;
use std::io;
use std::sync::mpsc::{self, Receiver, Sender, TryRecvError};
use std::thread;
use std::time::Duration;

use crossterm::event::{self, Event, KeyCode, KeyEvent, KeyModifiers};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::backend::CrosstermBackend;
use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span, Text};
use ratatui::widgets::{Paragraph, Wrap};
use ratatui::{Frame, Terminal};

use repomop_core::{
    Artifact, ScanOptions, format_bytes, relative_path_or_self,
    sort_artifacts_by_size_desc,
};
use repomop_fs::{DeleteResult, delete_artifacts};
use repomop_scanner::scan_and_measure;

const SPINNER_FRAMES: &[&str] = &["⠁", "⠂", "⠄", "⠂"];

#[derive(Debug, Clone, Default)]
pub struct SessionResult {
    pub fatal_error: Option<String>,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ViewState {
    Loading,
    List,
    Confirm,
    Deleting,
    Done,
    Error,
}

#[derive(Debug)]
enum BackgroundEvent {
    ScanFinished(Result<(Vec<Artifact>, Vec<String>), String>),
    DeleteFinished(DeleteResult),
}

#[derive(Debug)]
struct App {
    opts: ScanOptions,
    state: ViewState,
    artifacts: Vec<Artifact>,
    selected: BTreeSet<usize>,
    cached_selection: Option<Vec<Artifact>>,
    cursor: usize,
    confirm_offset: usize,
    scan_warnings: Vec<String>,
    delete_result: DeleteResult,
    message: String,
    fatal_error: Option<String>,
    spinner_index: usize,
    should_quit: bool,
    background_rx: Receiver<BackgroundEvent>,
    background_tx: Sender<BackgroundEvent>,
}

impl App {
    fn new(opts: ScanOptions) -> Self {
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
            spinner_index: 0,
            should_quit: false,
            background_rx,
            background_tx,
        };
        app.spawn_scan();
        app
    }

    fn spawn_scan(&self) {
        let sender = self.background_tx.clone();
        let opts = self.opts.clone();
        thread::spawn(move || {
            let _ =
                sender.send(BackgroundEvent::ScanFinished(scan_and_measure(&opts)));
        });
    }

    fn spawn_delete(&self, artifacts: Vec<Artifact>) {
        let sender = self.background_tx.clone();
        thread::spawn(move || {
            let result = delete_artifacts(&artifacts);
            let _ = sender.send(BackgroundEvent::DeleteFinished(result));
        });
    }

    fn drain_background(&mut self) {
        loop {
            match self.background_rx.try_recv() {
                Ok(BackgroundEvent::ScanFinished(result)) => match result {
                    Ok((artifacts, warnings)) => {
                        self.artifacts = artifacts;
                        self.scan_warnings = warnings;
                        self.selected.clear();
                        self.cursor = 0;
                        if self.artifacts.is_empty() {
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
                Err(TryRecvError::Empty) | Err(TryRecvError::Disconnected) => break,
            }
        }
    }

    fn tick(&mut self) {
        if matches!(self.state, ViewState::Loading | ViewState::Deleting) {
            self.spinner_index = (self.spinner_index + 1) % SPINNER_FRAMES.len();
        }
    }

    fn handle_key(&mut self, key: KeyEvent, area: Rect) {
        match self.state {
            ViewState::Loading => {
                if is_quit_key(key) {
                    self.should_quit = true;
                }
            }
            ViewState::List => self.handle_list_key(key),
            ViewState::Confirm => self.handle_confirm_key(key, area),
            ViewState::Deleting => {}
            ViewState::Done | ViewState::Error => {
                if is_quit_key(key) || key.code == KeyCode::Enter {
                    self.should_quit = true;
                }
            }
        }
    }

    fn handle_list_key(&mut self, key: KeyEvent) {
        match key.code {
            KeyCode::Up | KeyCode::Char('k') => {
                self.cursor = self.cursor.saturating_sub(1);
            }
            KeyCode::Down | KeyCode::Char('j') => {
                if self.cursor + 1 < self.artifacts.len() {
                    self.cursor += 1;
                }
            }
            KeyCode::Char(' ') => {
                if self.artifacts.is_empty() {
                    return;
                }
                if !self.selected.insert(self.cursor) {
                    self.selected.remove(&self.cursor);
                }
            }
            KeyCode::Enter => {
                if self.selected.is_empty() {
                    return;
                }
                self.cached_selection = Some(self.selected_artifacts());
                self.confirm_offset = 0;
                self.message.clear();
                self.state = ViewState::Confirm;
            }
            _ if is_quit_key(key) => self.should_quit = true,
            _ => {}
        }
    }

    fn handle_confirm_key(&mut self, key: KeyEvent, area: Rect) {
        let selected = self.confirm_artifacts();
        match key.code {
            KeyCode::Esc | KeyCode::Char('n') => {
                self.cached_selection = None;
                self.confirm_offset = 0;
                self.state = ViewState::List;
            }
            KeyCode::Up | KeyCode::Char('k') => {
                self.confirm_offset = self.confirm_offset.saturating_sub(1);
            }
            KeyCode::Down | KeyCode::Char('j') => {
                let max_offset =
                    max_confirm_offset(selected.len(), area.height as usize);
                if self.confirm_offset < max_offset {
                    self.confirm_offset += 1;
                }
            }
            KeyCode::Char('y') => {
                if selected.is_empty() {
                    self.cached_selection = None;
                    self.confirm_offset = 0;
                    self.state = ViewState::List;
                } else {
                    self.cached_selection = None;
                    self.confirm_offset = 0;
                    self.message.clear();
                    self.state = ViewState::Deleting;
                    self.spawn_delete(selected);
                }
            }
            KeyCode::Enter => {}
            _ if is_quit_key(key) => self.should_quit = true,
            _ => {}
        }
    }

    fn selected_count(&self) -> usize {
        self.selected.len()
    }

    fn selected_size(&self) -> u64 {
        self.selected
            .iter()
            .filter_map(|index| self.artifacts.get(*index))
            .map(|artifact| artifact.size_bytes)
            .sum()
    }

    fn selected_artifacts(&self) -> Vec<Artifact> {
        let mut artifacts: Vec<_> = self
            .selected
            .iter()
            .filter_map(|index| self.artifacts.get(*index).cloned())
            .collect();
        sort_artifacts_by_size_desc(&mut artifacts);
        artifacts
    }

    fn confirm_artifacts(&self) -> Vec<Artifact> {
        self.cached_selection.clone().unwrap_or_else(|| self.selected_artifacts())
    }

    fn render(&self, frame: &mut Frame) {
        let area = frame.area();

        match self.state {
            ViewState::Loading => self.render_loading(frame, area),
            ViewState::List => self.render_list(frame, area),
            ViewState::Confirm => self.render_confirm(frame, area),
            ViewState::Deleting => self.render_deleting(frame, area),
            ViewState::Done => self.render_done(frame, area),
            ViewState::Error => self.render_error(frame, area),
        }
    }

    fn render_loading(&self, frame: &mut Frame, area: Rect) {
        let text = Text::from(vec![
            styled_line("repomop", Color::White, true),
            Line::raw(""),
            Line::raw(format!(
                "{} Scanning directories and calculating sizes...",
                spinner(self.spinner_index)
            )),
            Line::raw("Press q to quit."),
        ]);
        frame.render_widget(paragraph(text), area);
    }

    fn render_list(&self, frame: &mut Frame, area: Rect) {
        let mut lines = vec![
            styled_line(
                format!("Found {} artifacts", self.artifacts.len()),
                Color::Gray,
                false,
            ),
            styled_line(
                format!(
                    "Selected {} · Reclaimable {}",
                    self.selected_count(),
                    format_bytes(self.selected_size() as i64)
                ),
                Color::Blue,
                true,
            ),
            Line::raw(""),
        ];

        let (start, end) = visible_range_for(
            self.artifacts.len(),
            self.cursor,
            area.height as usize,
            6,
            5,
        );
        let size_width = compute_size_column_width(&self.artifacts);
        for index in start..end {
            lines.push(self.render_artifact_line(
                index,
                area.width as usize,
                size_width,
            ));
        }

        if !self.message.is_empty() {
            lines.push(Line::raw(""));
            lines.push(styled_line(self.message.clone(), Color::Yellow, false));
        }

        lines.push(Line::raw(""));
        let help = if self.selected_count() > 0 {
            "↑↓ | Space select | Enter confirm | q quit"
        } else {
            "↑↓ | Space select | q quit"
        };
        lines.push(styled_line(help, Color::DarkGray, false));

        frame.render_widget(paragraph(Text::from(lines)), area);
    }

    fn render_confirm(&self, frame: &mut Frame, area: Rect) {
        let selected = self.confirm_artifacts();
        let total_size: u64 =
            selected.iter().map(|artifact| artifact.size_bytes).sum();

        let mut lines = vec![
            styled_line("Delete selected artifacts?", Color::White, true),
            Line::raw(""),
            Line::raw(if selected.len() == 1 {
                "1 artifact will be permanently deleted".to_string()
            } else {
                format!("{} artifacts will be permanently deleted", selected.len())
            }),
            Line::from(vec![
                Span::raw("Estimated space to free: "),
                Span::styled(
                    format_bytes(total_size as i64),
                    Style::default().fg(Color::Cyan),
                ),
            ]),
            Line::raw(""),
            styled_line("This action cannot be undone.", Color::Yellow, false),
            Line::raw(""),
            Line::raw("Items to delete:"),
        ];

        let (start, end) = visible_range_from_offset(
            selected.len(),
            self.confirm_offset,
            area.height as usize,
            11,
            3,
        );
        for artifact in &selected[start..end] {
            let relative =
                relative_path_or_self(&self.opts.root_path, &artifact.path)
                    .display()
                    .to_string();
            let size = format!("({})", format_bytes(artifact.size_bytes as i64));
            let width = area.width as usize;
            let path_width = width.saturating_sub(size.len() + 3).max(1);
            lines.push(Line::from(vec![
                Span::raw("- "),
                Span::styled(
                    truncate_path_left(&relative, path_width),
                    Style::default().fg(Color::White),
                ),
                Span::raw(" "),
                Span::styled(size, Style::default().fg(Color::Cyan)),
            ]));
        }

        lines.push(Line::raw(""));
        lines.push(Line::raw("Press Y to delete, or N to cancel: [y/N]"));
        lines.push(styled_line(
            "↑↓ Review items | y delete | n/esc cancel | q quit",
            Color::DarkGray,
            false,
        ));

        frame.render_widget(paragraph(Text::from(lines)), area);
    }

    fn render_deleting(&self, frame: &mut Frame, area: Rect) {
        let text = Text::from(vec![
            styled_line("repomop", Color::White, true),
            Line::raw(""),
            Line::raw(format!(
                "{} Removing selected artifacts...",
                spinner(self.spinner_index)
            )),
        ]);
        frame.render_widget(paragraph(text), area);
    }

    fn render_done(&self, frame: &mut Frame, area: Rect) {
        if self.fatal_error.is_some() {
            self.render_error(frame, area);
            return;
        }

        let mut lines = if self.delete_result.deleted.is_empty()
            && self.message == "No artifacts found."
        {
            vec![
                styled_line("repomop", Color::White, true),
                Line::raw(""),
                styled_line("No artifacts found.", Color::Green, false),
            ]
        } else {
            vec![
                styled_line("repomop: done", Color::White, true),
                Line::raw(""),
                styled_line(
                    format!(
                        "Deleted artifacts: {}",
                        self.delete_result.deleted.len()
                    ),
                    Color::Green,
                    false,
                ),
                styled_line(
                    format!(
                        "Freed space: {}",
                        format_bytes(self.delete_result.freed_bytes as i64)
                    ),
                    Color::Cyan,
                    true,
                ),
            ]
        };

        if !self.delete_result.errors.is_empty() {
            lines.push(Line::raw(""));
            lines.push(styled_line("Errors:", Color::Red, true));
            for item in &self.delete_result.errors {
                lines.push(styled_line(
                    format!(
                        "- {}: {}",
                        relative_path_or_self(
                            &self.opts.root_path,
                            &item.artifact.path
                        )
                        .display(),
                        item.error
                    ),
                    Color::Red,
                    false,
                ));
            }
        }

        if !self.scan_warnings.is_empty() {
            lines.push(Line::raw(""));
            lines.push(styled_line(
                format!("Size warnings: {}", self.scan_warnings.len()),
                Color::Yellow,
                false,
            ));
        }

        if !self.message.is_empty() && self.message != "No artifacts found." {
            lines.push(Line::raw(""));
            lines.push(styled_line(self.message.clone(), Color::Yellow, false));
        }

        lines.push(Line::raw(""));
        lines.push(styled_line("Press Enter to exit.", Color::DarkGray, false));

        frame.render_widget(paragraph(Text::from(lines)), area);
    }

    fn render_error(&self, frame: &mut Frame, area: Rect) {
        let text = Text::from(vec![
            styled_line("repomop: error", Color::White, true),
            Line::raw(""),
            styled_line(self.message.clone(), Color::Red, false),
            Line::raw(""),
            styled_line("Press Enter to exit.", Color::DarkGray, false),
        ]);
        frame.render_widget(paragraph(text), area);
    }

    fn render_artifact_line(
        &self,
        index: usize,
        width: usize,
        size_width: usize,
    ) -> Line<'static> {
        let artifact = &self.artifacts[index];
        let focused = self.cursor == index;
        let marker = if self.selected.contains(&index) { '●' } else { '○' };
        let bar = if focused { "▶ " } else { "  " };
        let size = format!(
            "{:>width$}",
            format_bytes(artifact.size_bytes as i64),
            width = size_width
        );
        let kind = artifact.kind.to_string();
        let relative = relative_path_or_self(&self.opts.root_path, &artifact.path)
            .display()
            .to_string();
        let path_width = path_column_width(width, size_width, kind.len());
        let path = truncate_path_left(&relative, path_width);

        Line::from(vec![
            Span::styled(bar.to_string(), focus_style(Style::default(), focused)),
            Span::styled(
                marker.to_string(),
                focus_style(marker_style(index, &self.selected), focused),
            ),
            Span::styled("  ", focus_style(Style::default(), focused)),
            Span::styled(
                size,
                focus_style(Style::default().fg(Color::Cyan), focused),
            ),
            Span::styled("  ", focus_style(Style::default(), focused)),
            Span::styled(
                path,
                focus_style(Style::default().fg(Color::White), focused),
            ),
            Span::styled("  ", focus_style(Style::default(), focused)),
            Span::styled(
                kind,
                focus_style(Style::default().fg(Color::DarkGray), focused),
            ),
        ])
    }
}

pub fn run(opts: ScanOptions) -> Result<SessionResult, String> {
    enable_raw_mode().map_err(|err| err.to_string())?;
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen).map_err(|err| err.to_string())?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend).map_err(|err| err.to_string())?;

    let result = run_app(&mut terminal, opts);

    let cleanup_result = (|| -> Result<(), String> {
        disable_raw_mode().map_err(|err| err.to_string())?;
        execute!(terminal.backend_mut(), LeaveAlternateScreen)
            .map_err(|err| err.to_string())?;
        terminal.show_cursor().map_err(|err| err.to_string())?;
        Ok(())
    })();

    match (result, cleanup_result) {
        (Ok(outcome), Ok(())) => Ok(outcome),
        (Err(err), Ok(())) => Err(err),
        (Ok(_), Err(err)) => Err(err),
        (Err(primary), Err(cleanup)) => {
            Err(format!("{primary}; cleanup failed: {cleanup}"))
        }
    }
}

fn run_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    opts: ScanOptions,
) -> Result<SessionResult, String> {
    let mut app = App::new(opts);

    loop {
        app.drain_background();
        terminal.draw(|frame| app.render(frame)).map_err(|err| err.to_string())?;

        if app.should_quit {
            return Ok(SessionResult { fatal_error: app.fatal_error.clone() });
        }

        if event::poll(Duration::from_millis(90)).map_err(|err| err.to_string())? {
            if let Event::Key(key) = event::read().map_err(|err| err.to_string())? {
                let area = terminal.get_frame().area();
                app.handle_key(key, area);
            }
        } else {
            app.tick();
        }
    }
}

fn paragraph(text: Text<'static>) -> Paragraph<'static> {
    Paragraph::new(text).wrap(Wrap { trim: false })
}

fn styled_line<T: Into<String>>(text: T, color: Color, bold: bool) -> Line<'static> {
    let style = if bold {
        Style::default().fg(color).add_modifier(Modifier::BOLD)
    } else {
        Style::default().fg(color)
    };
    Line::from(Span::styled(text.into(), style))
}

fn spinner(index: usize) -> &'static str {
    SPINNER_FRAMES[index % SPINNER_FRAMES.len()]
}

fn focus_style(style: Style, focused: bool) -> Style {
    if focused { style.add_modifier(Modifier::BOLD) } else { style }
}

fn marker_style(index: usize, selected: &BTreeSet<usize>) -> Style {
    if selected.contains(&index) {
        Style::default().fg(Color::Green)
    } else {
        Style::default().fg(Color::Gray)
    }
}

fn is_quit_key(key: KeyEvent) -> bool {
    matches!(key.code, KeyCode::Esc | KeyCode::Char('q'))
        || (key.code == KeyCode::Char('c')
            && key.modifiers.contains(KeyModifiers::CONTROL))
}

fn visible_range_for(
    total: usize,
    cursor: usize,
    height: usize,
    chrome_lines: usize,
    min_list_height: usize,
) -> (usize, usize) {
    if total == 0 {
        return (0, 0);
    }

    let list_height = list_height_for(total, height, chrome_lines, min_list_height);
    let cursor = cursor.min(total.saturating_sub(1));
    let mut start = cursor.saturating_sub(list_height.saturating_sub(1));
    let mut end = start + list_height;
    if end > total {
        end = total;
        start = end.saturating_sub(list_height);
    }
    (start, end)
}

fn visible_range_from_offset(
    total: usize,
    offset: usize,
    height: usize,
    chrome_lines: usize,
    min_list_height: usize,
) -> (usize, usize) {
    if total == 0 {
        return (0, 0);
    }

    let list_height = list_height_for(total, height, chrome_lines, min_list_height);
    let max_offset = total.saturating_sub(list_height);
    let offset = offset.min(max_offset);
    (offset, offset + list_height)
}

fn list_height_for(
    total: usize,
    height: usize,
    chrome_lines: usize,
    min_list_height: usize,
) -> usize {
    let visible = height.saturating_sub(chrome_lines).max(min_list_height);
    visible.min(total)
}

fn max_confirm_offset(total: usize, height: usize) -> usize {
    let list_height = list_height_for(total, height, 11, 3);
    total.saturating_sub(list_height)
}

fn compute_size_column_width(artifacts: &[Artifact]) -> usize {
    artifacts
        .iter()
        .map(|artifact| format_bytes(artifact.size_bytes as i64).len())
        .max()
        .unwrap_or_else(|| format_bytes(0).len())
}

fn path_column_width(
    total_width: usize,
    size_width: usize,
    kind_width: usize,
) -> usize {
    total_width.saturating_sub(2 + 1 + 6 + size_width + kind_width).max(1)
}

fn truncate_path_left(path: &str, max_width: usize) -> String {
    const ELLIPSIS: &str = "…";
    if max_width == 0 {
        return String::new();
    }
    if path.chars().count() <= max_width {
        return path.to_string();
    }
    if max_width == 1 {
        return ELLIPSIS.to_string();
    }

    let prefix = if path.contains(std::path::MAIN_SEPARATOR) {
        format!("{ELLIPSIS}{}", std::path::MAIN_SEPARATOR)
    } else {
        ELLIPSIS.to_string()
    };
    if prefix.chars().count() >= max_width {
        return ELLIPSIS.to_string();
    }

    let remaining = max_width - prefix.chars().count();
    let tail: String = path
        .chars()
        .rev()
        .take(remaining)
        .collect::<Vec<_>>()
        .into_iter()
        .rev()
        .collect();
    format!("{prefix}{tail}")
}

#[cfg(test)]
mod tests {
    use super::{truncate_path_left, visible_range_for};

    #[test]
    fn truncate_keeps_short_paths() {
        assert_eq!(truncate_path_left("short", 20), "short");
    }

    #[test]
    fn truncate_uses_ellipsis() {
        let value = truncate_path_left("a/very/long/path/node_modules", 12);
        assert!(value.starts_with("…/"));
        assert!(value.ends_with("modules"));
    }

    #[test]
    fn visible_range_handles_small_heights() {
        let (start, end) = visible_range_for(20, 0, 5, 6, 5);
        assert_eq!(start, 0);
        assert_eq!(end - start, 5);
    }
}
