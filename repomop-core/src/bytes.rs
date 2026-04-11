const BYTE_UNITS: [&str; 6] = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];

pub fn format_bytes(value: i64) -> String {
    if value < 0 {
        return "0 B".to_string();
    }
    if value < 1024 {
        return format!("{value} B");
    }

    let mut size = value as f64;
    let mut unit = 0usize;
    while size >= 1024.0 && unit < BYTE_UNITS.len() - 1 {
        size /= 1024.0;
        unit += 1;
    }

    format!("{size:.1} {}", BYTE_UNITS[unit])
}

#[cfg(test)]
mod tests {
    use super::format_bytes;

    #[test]
    fn formats_negative_as_zero() {
        assert_eq!(format_bytes(-1), "0 B");
    }

    #[test]
    fn formats_small_values() {
        assert_eq!(format_bytes(0), "0 B");
        assert_eq!(format_bytes(1), "1 B");
        assert_eq!(format_bytes(1023), "1023 B");
    }

    #[test]
    fn formats_binary_units() {
        assert_eq!(format_bytes(1024), "1.0 KiB");
        assert_eq!(format_bytes(1536), "1.5 KiB");
        assert_eq!(format_bytes(1024 * 1024), "1.0 MiB");
        assert_eq!(format_bytes(1024 * 1024 * 1024), "1.0 GiB");
    }
}
