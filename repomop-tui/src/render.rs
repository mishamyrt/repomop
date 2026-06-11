use std::collections::BTreeSet;

use ratatui::Frame;
use ratatui::layout::Rect;
use ratatui::style::{Color, Style};
use ratatui::text::{Line, Span, Text};

use repomop_core::format_bytes;

use crate::app::{App, ViewState};
use crate::widgets::{
    CONFIRM_CHROME_LINES, CONFIRM_MIN_VISIBLE, LIST_CHROME_LINES, LIST_MIN_VISIBLE,
    artifact_span, compute_kind_column_width, compute_list_path_column_width,
    compute_size_column_width, delete_spinner, display_relative, focus_style,
    pad_left_to_width, pad_right_to_width, paragraph, scan_spinner, styled_line,
    truncate_path_left, visible_range_for, visible_range_from_offset,
};

impl App {
    pub(crate) fn render(&self, frame: &mut Frame) {
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
            styled_line("repomop", self.theme.primary_text(), true),
            Line::raw(""),
            Line::raw(format!(
                "{} Scanning directories and calculating sizes...",
                scan_spinner(self.scan_spinner_index)
            )),
            styled_line("Press q to quit.", Color::DarkGray, false),
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
                    format_bytes(self.selected_size())
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
            LIST_CHROME_LINES,
            LIST_MIN_VISIBLE,
        );
        let size_width = compute_size_column_width(&self.artifacts);
        let kind_width = compute_kind_column_width(&self.artifacts);
        let path_width = compute_list_path_column_width(
            &self.opts.root_path,
            &self.artifacts,
            area.width as usize,
            size_width,
            kind_width,
        );
        for index in start..end {
            lines.push(
                self.render_artifact_line(index, size_width, path_width, kind_width),
            );
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
            styled_line(
                "Delete selected artifacts?",
                self.theme.primary_text(),
                true,
            ),
            Line::raw(""),
            Line::raw(if selected.len() == 1 {
                "1 artifact will be permanently deleted".to_string()
            } else {
                format!("{} artifacts will be permanently deleted", selected.len())
            }),
            Line::from(vec![
                Span::raw("Estimated space to free: "),
                Span::styled(
                    format_bytes(total_size),
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
            CONFIRM_CHROME_LINES,
            CONFIRM_MIN_VISIBLE,
        );
        for artifact in &selected[start..end] {
            let relative = display_relative(&self.opts.root_path, &artifact.path);
            let size = format!("({})", format_bytes(artifact.size_bytes));
            let width = area.width as usize;
            let path_width = width.saturating_sub(size.len() + 3).max(1);
            lines.push(Line::from(vec![
                Span::raw("- "),
                Span::styled(
                    truncate_path_left(&relative, path_width),
                    Style::default().fg(self.theme.primary_text()),
                ),
                Span::raw(" "),
                Span::styled(size, Style::default().fg(Color::Cyan)),
            ]));
        }

        lines.push(Line::raw(""));
        lines.push(Line::raw("Press Y to delete"));
        lines.push(styled_line(
            "↑↓ Review items | y delete | n/esc cancel | q quit",
            Color::DarkGray,
            false,
        ));

        frame.render_widget(paragraph(Text::from(lines)), area);
    }

    fn render_deleting(&self, frame: &mut Frame, area: Rect) {
        let text = Text::from(vec![
            styled_line("repomop", self.theme.primary_text(), true),
            Line::raw(""),
            Line::raw(format!(
                "{} Removing selected artifacts...",
                delete_spinner(self.delete_spinner_index)
            )),
        ]);
        frame.render_widget(paragraph(text), area);
    }

    fn render_done(&self, frame: &mut Frame, area: Rect) {
        if self.fatal_error.is_some() {
            self.render_error(frame, area);
            return;
        }

        let mut lines = if self.no_artifacts_found {
            vec![
                styled_line("repomop", self.theme.primary_text(), true),
                Line::raw(""),
                styled_line("No artifacts found.", Color::Green, false),
            ]
        } else {
            vec![
                styled_line("repomop: done", self.theme.primary_text(), true),
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
                        format_bytes(self.delete_result.freed_bytes)
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
                        display_relative(&self.opts.root_path, &item.artifact.path,),
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

        if !self.message.is_empty() && !self.no_artifacts_found {
            lines.push(Line::raw(""));
            lines.push(styled_line(self.message.clone(), Color::Yellow, false));
        }

        lines.push(Line::raw(""));
        lines.push(styled_line("Press Enter to exit.", Color::DarkGray, false));

        frame.render_widget(paragraph(Text::from(lines)), area);
    }

    fn render_error(&self, frame: &mut Frame, area: Rect) {
        let text = Text::from(vec![
            styled_line("repomop: error", self.theme.primary_text(), true),
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
        size_width: usize,
        path_width: usize,
        kind_width: usize,
    ) -> Line<'static> {
        let artifact = &self.artifacts[index];
        let focused = self.cursor == index;
        let marker = if self.selected.contains(&index) { '●' } else { '○' };
        let bar = if focused { "▶ " } else { "  " };
        let size = pad_left_to_width(format_bytes(artifact.size_bytes), size_width);
        let kind = artifact.kind.to_string();
        let relative = display_relative(&self.opts.root_path, &artifact.path);
        let path = pad_right_to_width(
            truncate_path_left(&relative, path_width),
            path_width,
        );
        let kind = format!("{kind:>kind_width$}");

        Line::from(vec![
            span_focus(bar.to_string(), focused),
            span_focus_colored(
                marker.to_string(),
                marker_color(index, &self.selected),
                focused,
            ),
            span_focus("  ".to_string(), focused),
            artifact_span(size, Color::Cyan, focused),
            span_focus("  ".to_string(), focused),
            artifact_span(path, self.theme.primary_text(), focused),
            span_focus("  ".to_string(), focused),
            artifact_span(kind, Color::DarkGray, focused),
        ])
    }
}

fn span_focus(text: String, focused: bool) -> Span<'static> {
    Span::styled(text, focus_style(Style::default(), focused))
}

fn span_focus_colored(text: String, color: Color, focused: bool) -> Span<'static> {
    Span::styled(text, focus_style(Style::default().fg(color), focused))
}

fn marker_color(index: usize, selected: &BTreeSet<usize>) -> Color {
    if selected.contains(&index) { Color::Green } else { Color::Gray }
}
