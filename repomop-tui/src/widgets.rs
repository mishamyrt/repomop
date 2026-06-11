use std::path::Path;

use ratatui::style::{Color, Modifier, Style};
use ratatui::text::Text;
use ratatui::text::{Line, Span};
use ratatui::widgets::{Paragraph, Wrap};
use unicode_width::{UnicodeWidthChar, UnicodeWidthStr};

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

const SCAN_SPINNER_FRAMES: [&str; 10] =
    ["⠀⠀⠀⠀", "⡇⠀⠀⠀", "⣿⠀⠀⠀", "⢸⡇⠀⠀", "⠀⣿⠀⠀", "⠀⢸⡇⠀", "⠀⠀⣿⠀", "⠀⠀⢸⡇", "⠀⠀⠀⣿", "⠀⠀⠀⢸"];
const DELETE_SPINNER_FRAMES: [&str; 16] = [
    "⠁⠀", "⠋⠀", "⠟⠁", "⡿⠋", "⣿⠟", "⣿⡿", "⣿⣿", "⣿⣿", "⣾⣿", "⣴⣿", "⣠⣾", "⢀⣴", "⠀⣠",
    "⠀⢀", "⠀⠀", "⠀⠀",
];

pub(crate) const SCAN_SPINNER_FRAME_COUNT: usize = SCAN_SPINNER_FRAMES.len();
pub(crate) const DELETE_SPINNER_FRAME_COUNT: usize = DELETE_SPINNER_FRAMES.len();

fn spinner_frame<const N: usize>(
    frames: &[&'static str; N],
    index: usize,
) -> &'static str {
    frames[index % N]
}

pub(crate) fn scan_spinner(index: usize) -> &'static str {
    spinner_frame(&SCAN_SPINNER_FRAMES, index)
}

pub(crate) fn delete_spinner(index: usize) -> &'static str {
    spinner_frame(&DELETE_SPINNER_FRAMES, index)
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

const WIDE_PATH_COLUMN_MAX: usize = 72;

pub(crate) fn compute_size_column_width(artifacts: &[Artifact]) -> usize {
    artifacts
        .iter()
        .map(|artifact| display_width(&format_bytes(artifact.size_bytes)))
        .max()
        .unwrap_or_else(|| format_bytes(0).len())
}

pub(crate) fn compute_kind_column_width(artifacts: &[Artifact]) -> usize {
    artifacts.iter().map(|artifact| artifact.kind.as_str().len()).max().unwrap_or(0)
}

pub(crate) fn compute_list_path_column_width(
    root: &Path,
    artifacts: &[Artifact],
    total_width: usize,
    size_width: usize,
    kind_width: usize,
) -> usize {
    let available_width = path_column_width(total_width, size_width, kind_width);
    if total_width <= 100 {
        return available_width;
    }

    artifacts
        .iter()
        .map(|artifact| display_width(&display_relative(root, &artifact.path)))
        .max()
        .unwrap_or(1)
        .min(WIDE_PATH_COLUMN_MAX)
        .min(available_width)
        .max(1)
}

pub(crate) fn path_column_width(
    total_width: usize,
    size_width: usize,
    kind_width: usize,
) -> usize {
    let fixed_width = display_width("▶ ")
        + display_width("●")
        + display_width("  ")
        + display_width("  ")
        + display_width("  ");
    total_width.saturating_sub(fixed_width + size_width + kind_width).max(1)
}

pub(crate) fn display_width(text: &str) -> usize {
    UnicodeWidthStr::width(text)
}

pub(crate) fn pad_right_to_width(mut text: String, target_width: usize) -> String {
    let width = display_width(&text);
    if width < target_width {
        text.push_str(&" ".repeat(target_width - width));
    }
    text
}

pub(crate) fn pad_left_to_width(text: String, target_width: usize) -> String {
    let width = display_width(&text);
    if width < target_width {
        format!("{}{}", " ".repeat(target_width - width), text)
    } else {
        text
    }
}

// ── Path truncation ───────────────────────────────────────────────────────────

pub(crate) fn truncate_path_left(path: &str, max_width: usize) -> String {
    const ELLIPSIS: &str = "…";
    if max_width == 0 {
        return String::new();
    }
    if display_width(path) <= max_width {
        return path.to_string();
    }
    if max_width <= display_width(ELLIPSIS) {
        return ELLIPSIS.to_string();
    }

    let prefix = if path.contains(std::path::MAIN_SEPARATOR) {
        format!("{ELLIPSIS}{}", std::path::MAIN_SEPARATOR)
    } else {
        ELLIPSIS.to_string()
    };
    let prefix_width = display_width(&prefix);
    if prefix_width >= max_width {
        return ELLIPSIS.to_string();
    }

    let tail = tail_for_width(path, max_width - prefix_width);
    format!("{prefix}{tail}")
}

fn tail_for_width(text: &str, max_width: usize) -> String {
    let mut width = 0;
    let mut tail = Vec::new();
    for ch in text.chars().rev() {
        let char_width = UnicodeWidthChar::width(ch).unwrap_or(0);
        if width + char_width > max_width {
            break;
        }
        width += char_width;
        tail.push(ch);
    }
    tail.into_iter().rev().collect()
}

// ── Path display ──────────────────────────────────────────────────────────────

/// Formats a path relative to `root` (without allocation when possible).
pub(crate) fn display_relative(root: &Path, path: &Path) -> String {
    repomop_core::relative_path_or_self(root, path).display().to_string()
}

#[cfg(test)]
mod tests {
    use super::{
        DELETE_SPINNER_FRAME_COUNT, SCAN_SPINNER_FRAME_COUNT,
        compute_list_path_column_width, compute_size_column_width, delete_spinner,
        display_width, pad_left_to_width, pad_right_to_width, scan_spinner,
        truncate_path_left, visible_range_for,
    };
    use repomop_core::{Artifact, ArtifactKind};
    use std::path::PathBuf;

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

    #[test]
    fn list_path_column_uses_remaining_width_on_narrow_screens() {
        let artifacts = vec![artifact("/repo/short")];

        assert_eq!(
            compute_list_path_column_width(
                PathBuf::from("/repo").as_path(),
                &artifacts,
                80,
                8,
                12
            ),
            51
        );
    }

    #[test]
    fn list_path_column_uses_content_width_on_wide_screens() {
        let artifacts = vec![artifact("/repo/short")];

        assert_eq!(
            compute_list_path_column_width(
                PathBuf::from("/repo").as_path(),
                &artifacts,
                120,
                8,
                12
            ),
            5
        );
    }

    #[test]
    fn list_path_column_caps_long_paths_on_wide_screens() {
        let artifacts = vec![artifact(
            "/repo/a/very/long/path/that/should/not/push/the/type/to/the/right/edge/node_modules",
        )];

        assert_eq!(
            compute_list_path_column_width(
                PathBuf::from("/repo").as_path(),
                &artifacts,
                140,
                8,
                12
            ),
            72
        );
    }

    #[test]
    fn truncate_respects_terminal_display_width() {
        let value = truncate_path_left("a/界界界/node_modules", 10);

        assert!(display_width(&value) <= 10);
        assert!(value.starts_with("…/"));
    }

    #[test]
    fn pad_right_respects_terminal_display_width() {
        let value = pad_right_to_width("界".to_string(), 4);

        assert_eq!(display_width(&value), 4);
        assert_eq!(value, "界  ");
    }

    #[test]
    fn size_column_uses_widest_formatted_size() {
        let artifacts = vec![
            artifact_with_size("/repo/target", 1024 * 1024 * 1024),
            artifact_with_size("/repo/node_modules", 999 * 1024 * 1024),
        ];

        assert_eq!(compute_size_column_width(&artifacts), "999.0 MiB".len());
    }

    #[test]
    fn list_row_path_start_does_not_depend_on_size_width() {
        let size_width = "1023.0 GiB".len();
        let short_size_row =
            plain_list_row("  ", "○", "1 B", "short", "node-modules", size_width);
        let long_size_row = plain_list_row(
            "  ",
            "○",
            "1023.0 GiB",
            "short",
            "node-modules",
            size_width,
        );

        assert_eq!(short_size_row.find("short"), long_size_row.find("short"));
    }

    #[test]
    fn scan_spinner_wraps_to_first_frame() {
        assert_eq!(scan_spinner(0), scan_spinner(SCAN_SPINNER_FRAME_COUNT));
    }

    #[test]
    fn delete_spinner_wraps_to_first_frame() {
        assert_eq!(delete_spinner(0), delete_spinner(DELETE_SPINNER_FRAME_COUNT));
    }

    fn artifact(path: &str) -> Artifact {
        artifact_with_size(path, 0)
    }

    fn artifact_with_size(path: &str, size_bytes: u64) -> Artifact {
        Artifact {
            kind: ArtifactKind::NodeModules,
            path: PathBuf::from(path),
            project_root: PathBuf::from("/repo"),
            size_bytes,
        }
    }

    fn plain_list_row(
        bar: &str,
        marker: &str,
        size: &str,
        path: &str,
        kind: &str,
        size_width: usize,
    ) -> String {
        let size = pad_left_to_width(size.to_string(), size_width);
        format!("{bar}{marker}  {size}  {path}  {kind}")
    }
}
