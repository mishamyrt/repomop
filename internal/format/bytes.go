package format

import (
	"strconv"
)

var byteUnits = []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}

// Bytes returns a human-readable binary size string.
func Bytes(value int64) string {
	if value < 0 {
		return "0 B"
	}
	if value < 1024 {
		return strconv.FormatInt(value, 10) + " B"
	}

	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(byteUnits)-1 {
		size /= 1024
		unit++
	}

	return strconv.FormatFloat(size, 'f', 1, 64) + " " + byteUnits[unit]
}
