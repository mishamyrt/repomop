package format

import "fmt"

var byteUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

// Bytes returns a human-readable binary size string.
func Bytes(value int64) string {
	if value < 0 {
		return "0 B"
	}
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}

	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(byteUnits)-1 {
		size /= 1024
		unit++
	}

	return fmt.Sprintf("%.1f %s", size, byteUnits[unit])
}
