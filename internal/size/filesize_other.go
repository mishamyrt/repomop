//go:build !unix

package size

import "io/fs"

// reclaimableFileSize returns the apparent file size on platforms where
// hard-link detection via syscall is unavailable.
func reclaimableFileSize(_ string, info fs.FileInfo) int64 {
	return info.Size()
}
