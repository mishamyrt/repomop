use std::env;
use std::io::{self, IsTerminal, Write};

use ratatui::style::Color;

const THEME_OVERRIDE_ENV: &str = "REPOMOP_THEME";
const COLORFGBG_ENV: &str = "COLORFGBG";

#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub(crate) enum TerminalTheme {
    Light,
    #[default]
    Dark,
}

impl TerminalTheme {
    pub(crate) const fn primary_text(self) -> Color {
        match self {
            Self::Light => Color::Black,
            Self::Dark => Color::White,
        }
    }
}

pub(crate) fn detect_terminal_theme() -> TerminalTheme {
    detect_theme_override()
        .or_else(detect_from_colorfgbg)
        .or_else(detect_from_osc_11)
        .unwrap_or_default()
}

fn detect_theme_override() -> Option<TerminalTheme> {
    let value = env::var(THEME_OVERRIDE_ENV).ok()?;
    match value.trim().to_ascii_lowercase().as_str() {
        "light" => Some(TerminalTheme::Light),
        "dark" => Some(TerminalTheme::Dark),
        "auto" | "" => None,
        _ => None,
    }
}

fn detect_from_colorfgbg() -> Option<TerminalTheme> {
    let value = env::var(COLORFGBG_ENV).ok()?;
    parse_colorfgbg(&value)
}

fn parse_colorfgbg(value: &str) -> Option<TerminalTheme> {
    let background = value.split(';').next_back()?.trim().parse::<u16>().ok()?;
    match background {
        0..=6 | 8..=14 | 232..=243 => Some(TerminalTheme::Dark),
        7 | 15 | 244..=255 => Some(TerminalTheme::Light),
        _ => None,
    }
}

#[cfg(unix)]
#[allow(clippy::cast_possible_wrap)]
fn detect_from_osc_11() -> Option<TerminalTheme> {
    use std::fs::OpenOptions;
    use std::os::fd::AsRawFd;
    use std::time::{Duration, Instant};

    const OSC_11_QUERY: &[u8] = b"\x1b]11;?\x07";
    const MAX_RESPONSE_BYTES: usize = 128;
    const QUERY_TIMEOUT: Duration = Duration::from_millis(75);

    if !io::stdin().is_terminal() || !io::stdout().is_terminal() {
        return None;
    }

    let mut tty = OpenOptions::new().read(true).write(true).open("/dev/tty").ok()?;
    tty.write_all(OSC_11_QUERY).ok()?;
    tty.flush().ok()?;

    let fd = tty.as_raw_fd();
    let deadline = Instant::now() + QUERY_TIMEOUT;
    let mut response = Vec::with_capacity(64);

    while Instant::now() < deadline && response.len() < MAX_RESPONSE_BYTES {
        let remaining = deadline.saturating_duration_since(Instant::now());
        let mut read_set = unsafe { std::mem::zeroed::<libc::fd_set>() };
        unsafe {
            libc::FD_ZERO(&raw mut read_set);
            libc::FD_SET(fd, &raw mut read_set);
        }
        let mut timeout = libc::timeval {
            tv_sec: remaining.as_secs() as libc::time_t,
            tv_usec: remaining.subsec_micros() as libc::suseconds_t,
        };

        let ready = unsafe {
            libc::select(
                fd + 1,
                &raw mut read_set,
                std::ptr::null_mut(),
                std::ptr::null_mut(),
                &raw mut timeout,
            )
        };
        if ready < 0 {
            let err = io::Error::last_os_error();
            if err.kind() == io::ErrorKind::Interrupted {
                continue;
            }
            return None;
        }
        if ready == 0 {
            break;
        }

        let mut byte = 0u8;
        let read =
            unsafe { libc::read(fd, std::ptr::from_mut(&mut byte).cast(), 1) };
        if read < 0 {
            let err = io::Error::last_os_error();
            if err.kind() == io::ErrorKind::Interrupted {
                continue;
            }
            return None;
        }
        if read == 0 {
            break;
        }

        response.push(byte);
        if byte == b'\x07' || response.ends_with(b"\x1b\\") {
            break;
        }
    }

    parse_osc_11_response(&response)
}

#[cfg(not(unix))]
fn detect_from_osc_11() -> Option<TerminalTheme> {
    None
}

fn parse_osc_11_response(bytes: &[u8]) -> Option<TerminalTheme> {
    let start = bytes.windows(5).position(|window| window == b"\x1b]11;")?;
    let payload = &bytes[start + 5..];
    let end = payload
        .iter()
        .position(|byte| *byte == b'\x07')
        .or_else(|| payload.windows(2).position(|window| window == b"\x1b\\"))?;
    let value = std::str::from_utf8(&payload[..end]).ok()?;
    let rgb = value.strip_prefix("rgb:")?;

    let mut components = rgb.split('/');
    let red = parse_hex_component(components.next()?)?;
    let green = parse_hex_component(components.next()?)?;
    let blue = parse_hex_component(components.next()?)?;
    if components.next().is_some() {
        return None;
    }

    Some(theme_from_rgb(red, green, blue))
}

#[allow(clippy::cast_possible_truncation)]
fn parse_hex_component(component: &str) -> Option<u16> {
    let digits = component.len();
    if !(1..=4).contains(&digits) {
        return None;
    }

    let value = u32::from(u16::from_str_radix(component, 16).ok()?);
    let max = (1u32 << (digits * 4)) - 1;
    Some((value * u32::from(u16::MAX) / max) as u16)
}

fn theme_from_rgb(red: u16, green: u16, blue: u16) -> TerminalTheme {
    let luma = (299u32 * u32::from(red)
        + 587u32 * u32::from(green)
        + 114u32 * u32::from(blue))
        / 1000;
    if luma >= u32::from(u16::MAX) / 2 {
        TerminalTheme::Light
    } else {
        TerminalTheme::Dark
    }
}

#[cfg(test)]
mod tests {
    use super::{
        TerminalTheme, parse_colorfgbg, parse_hex_component, parse_osc_11_response,
    };

    #[test]
    fn colorfgbg_detects_light_background() {
        assert_eq!(parse_colorfgbg("15;0;7"), Some(TerminalTheme::Light));
    }

    #[test]
    fn colorfgbg_detects_dark_background() {
        assert_eq!(parse_colorfgbg("15;0;0"), Some(TerminalTheme::Dark));
    }

    #[test]
    fn hex_component_scales_short_values() {
        assert_eq!(parse_hex_component("f"), Some(u16::MAX));
        assert_eq!(parse_hex_component("80"), Some(0x8080));
    }

    #[test]
    fn osc_11_detects_light_background() {
        let response = b"\x1b]11;rgb:ffff/ffff/ffff\x07";
        assert_eq!(parse_osc_11_response(response), Some(TerminalTheme::Light));
    }

    #[test]
    fn osc_11_detects_dark_background() {
        let response = b"\x1b]11;rgb:0000/0000/0000\x1b\\";
        assert_eq!(parse_osc_11_response(response), Some(TerminalTheme::Dark));
    }
}
