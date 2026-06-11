use std::env;
use std::io::{self, Write};

use crossterm::execute;
use crossterm::terminal::SetTitle;

pub(crate) fn set_title(title: &str) {
    let _ = execute!(io::stdout(), SetTitle(title));
}

pub(crate) fn notify(title: &str, body: &str) {
    let Some(sequence) = notification_sequence(title, body) else {
        return;
    };
    let mut stdout = io::stdout();
    let _ = stdout.write_all(sequence.as_bytes());
    let _ = stdout.flush();
}

fn notification_sequence(title: &str, body: &str) -> Option<String> {
    let title = sanitize_notification_text(title);
    let body = sanitize_notification_text(body);
    if title.is_empty() && body.is_empty() {
        return None;
    }

    let term_program = env::var("TERM_PROGRAM").unwrap_or_default();
    Some(notification_sequence_for(&term_program, &title, &body))
}

fn notification_sequence_for(term_program: &str, title: &str, body: &str) -> String {
    match term_program.to_ascii_lowercase().as_str() {
        "iterm.app" | "vscode" => {
            format!("\x1b]9;{}\x07", notification_message(title, body))
        }
        "wezterm" => format!("\x1b]777;notify;{title};{body}\x07"),
        _ => format!("\x1b]777;notify;{title};{body}\x07"),
    }
}

fn notification_message(title: &str, body: &str) -> String {
    match (title.is_empty(), body.is_empty()) {
        (true, true) => String::new(),
        (true, false) => body.to_string(),
        (false, true) => title.to_string(),
        (false, false) => format!("{title}: {body}"),
    }
}

fn sanitize_notification_text(value: &str) -> String {
    value
        .chars()
        .filter_map(|ch| match ch {
            '\x07' | '\x1b' => None,
            '\n' | '\r' | '\t' | ';' => Some(' '),
            _ if ch.is_control() => None,
            _ => Some(ch),
        })
        .collect::<String>()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
}

#[cfg(test)]
mod tests {
    use super::{notification_sequence_for, sanitize_notification_text};

    #[test]
    fn sanitizes_notification_control_text() {
        assert_eq!(
            sanitize_notification_text("hello;\n\x1b]bad\x07 world"),
            "hello ]bad world"
        );
    }

    #[test]
    fn formats_iterm_notification() {
        assert_eq!(
            notification_sequence_for("iTerm.app", "repomop", "Scan complete"),
            "\x1b]9;repomop: Scan complete\x07"
        );
    }

    #[test]
    fn formats_osc_777_notification() {
        assert_eq!(
            notification_sequence_for("WezTerm", "repomop", "Files removed"),
            "\x1b]777;notify;repomop;Files removed\x07"
        );
    }
}
