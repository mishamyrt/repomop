//go:build unix && !darwin

package size

import (
	"io/fs"
	"syscall"
)

// reclaimableFileSize returns the file size only when it has no other hard
// links outside this tree (Nlink == 1). Files managed by pnpm are hard-linked
// to a global content-addressable store, so removing them from node_modules
// does not actually free the underlying disk blocks.
func reclaimableFileSize(_ string, info fs.FileInfo, includeLinks bool) int64 {
	if !includeLinks {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && stat.Nlink > 1 {
			return 0
		}
	}
	return info.Size()
}
