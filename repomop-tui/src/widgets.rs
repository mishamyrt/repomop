use std::path::Path;

use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Paragraph, Wrap};
use ratatui::text::Text;

use repomop_core::{Artifact, format_bytes};

// ── Layout constants ──────────────────────────────────────────────────────────

/// Fixed chrome lines in the list view (title + status + blank + help + blank).
pub(crate) const LIST_CHROME_LINES: usize = 6;
/// Minimum number of artifact rows visible in the list view.
pub(crate) const LIST_MIN_VISIBLE: usize = 5;
/// Fixed chrome lines in the confirm view.
pub(crate) const CONFIRM_CHROME_LINES: usize = 11;
/// Minimum number of artifact rows visible in the confirm view.
pub(crate) const CONFIRM_MIN_VISIBLE: usize = 3;

// ── Widget helpers ────────────────────────────────────────────────────────────

pub(crate) fn paragraph(text: Text<'static>) -> Paragraph<'static> {
    Paragraph::new(text).wrap(Wrap { trim: false })
}

pub(crate) fn styled_line<T: Into<String>>(
    text: T,
    color: Color,
    bold: bool,
) -> Line<'static> {
    let style = if bold {
        Style::default().fg(color).add_modifier(Modifier::BOLD)
    } else {
        Style::default().fg(color)
    };
    Line::from(Span::styled(text.into(), style))
}

pub(crate) fn spinner(index: usize) -> &'static str {
    const FRAMES: &[&str] = &["⠁", "⠂", "⠄", "⠂"];
    FRAMES[index % FRAMES.len()]
}

pub(crate) fn focus_style(style: Style, focused: bool) -> Style {
    if focused { style.add_modifier(Modifier::BOLD) } else { style }
}

/// Builds a single span with optional focus highlight and consistent styling.
pub(crate) fn artifact_span(
    text: String,
    color: Color,
    focused: bool,
) -> Span<'static> {
    Span::styled(text, focus_style(Style::default().fg(color), focused))
}

// ── Visible-range helpers ─────────────────────────────────────────────────────

pub(crate) fn visible_range_for(
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

pub(crate) fn visible_range_from_offset(
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

pub(crate) fn list_height_for(
    total: usize,
    height: usize,
    chrome_lines: usize,
    min_list_height: usize,
) -> usize {
    let visible = height.saturating_sub(chrome_lines).max(min_list_height);
    visible.min(total)
}

pub(crate) fn max_confirm_offset(total: usize, height: usize) -> usize {
    let list_height =
        list_height_for(total, height, CONFIRM_CHROME_LINES, CONFIRM_MIN_VISIBLE);
    total.saturating_sub(list_height)
}

// ── Column-width helpers ──────────────────────────────────────────────────────

pub(crate) fn compute_size_column_width(artifacts: &[Artifact]) -> usize {
    artifacts
        .iter()
        .map(|a| a.size_bytes)
        .max()
        .map(|max| format_bytes(max).len())
        .unwrap_or_else(|| format_bytes(0).len())
}

pub(crate) fn path_column_width(
    total_width: usize,
    size_width: usize,
    kind_width: usize,
) -> usize {
    total_width.saturating_sub(2 + 1 + 6 + size_width + kind_width).max(1)
}

// ── Path truncation ───────────────────────────────────────────────────────────

pub(crate) fn truncate_path_left(path: &str, max_width: usize) -> String {
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
    let tail: String = path.chars().rev().take(remaining).collect::<Vec<_>>().into_iter().rev().collect();
    format!("{prefix}{tail}")
}

// ── Path display ──────────────────────────────────────────────────────────────

/// Formats a path relative to `root` (without allocation when possible).
pub(crate) fn display_relative(root: &Path, path: &Path) -> String {
    repomop_core::relative_path_or_self(root, path).display().to_string()
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
