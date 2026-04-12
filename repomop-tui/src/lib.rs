mod app;
mod input;
mod render;
mod theme;
mod widgets;

use std::io;

use crossterm::event::{self, Event};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;

use repomop_core::ScanOptions;

pub use app::SessionResult;

use app::App;
use std::time::Duration;
use theme::detect_terminal_theme;

pub fn run(opts: ScanOptions) -> Result<SessionResult, String> {
    enable_raw_mode().map_err(|err| err.to_string())?;
    let theme = detect_terminal_theme();
    let mut stdout = io::stdout();
    execute!(stdout, EnterAlternateScreen).map_err(|err| err.to_string())?;
    let backend = CrosstermBackend::new(stdout);
    let mut terminal = Terminal::new(backend).map_err(|err| err.to_string())?;

    let _guard = TerminalGuard;
    run_app(&mut terminal, opts, theme)
}

fn run_app(
    terminal: &mut Terminal<CrosstermBackend<io::Stdout>>,
    opts: ScanOptions,
    theme: theme::TerminalTheme,
) -> Result<SessionResult, String> {
    let mut app = App::new(opts, theme);

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

/// Restores the terminal to its original state when dropped.
/// Ensures cleanup runs even if `run_app` panics.
struct TerminalGuard;

impl Drop for TerminalGuard {
    fn drop(&mut self) {
        let _ = disable_raw_mode();
        let _ = execute!(io::stdout(), LeaveAlternateScreen);
    }
}
