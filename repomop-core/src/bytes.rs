const BYTE_UNITS: [&str; 6] = ["B", "KiB", "MiB", "GiB", "TiB", "PiB"];
const UNIT_STEP: u128 = 1024;
const DISPLAY_SCALE: u128 = 10;

pub fn format_bytes(value: u64) -> String {
    if value < 1024 {
        return format!("{value} B");
    }

    let mut divisor = 1u128;
    let mut unit = 0usize;
    while u128::from(value) >= divisor * UNIT_STEP && unit < BYTE_UNITS.len() - 1 {
        divisor *= UNIT_STEP;
        unit += 1;
    }

    let scaled = (u128::from(value) * DISPLAY_SCALE + divisor / 2) / divisor;
    format!(
        "{}.{} {}",
        scaled / DISPLAY_SCALE,
        scaled % DISPLAY_SCALE,
        BYTE_UNITS[unit]
    )
}

pub fn format_signed_bytes(value: i64) -> String {
    match u64::try_from(value) {
        Ok(value) => format_bytes(value),
        Err(_) => "0 B".to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::{format_bytes, format_signed_bytes};

    #[test]
    fn formats_negative_as_zero() {
        assert_eq!(format_signed_bytes(-1), "0 B");
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
