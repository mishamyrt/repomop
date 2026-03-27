//go:build darwin

package size

import (
	"encoding/binary"
	"io/fs"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	// ATTR_CMNEXT_PRIVATESIZE goes in the forkattr field of attrlist.
	attrCmnextPrivatesize = 0x00000008
	privateSizeBufLen     = 4 + 8
	sysGetattrlist        = 220
)

// reclaimableFileSize returns the private (non-shared) byte count for a file.
// Files managed by some package managers (e.g. pnpm) may be APFS clones (copy-on-write)
// of the global content-addressable store. Deleting a clone does not free the shared extents,
// so the reclaimable space is only the private bytes. Hard-linked files
// (Nlink > 1) are also excluded because the data survives in the store.
func reclaimableFileSize(path string, info fs.FileInfo, includeLinks bool) int64 {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size()
	}
	if !includeLinks && stat.Nlink > 1 {
		return 0
	}
	ps, err := privateDataSize(path)
	if err != nil {
		return info.Size()
	}
	apparent := info.Size()
	if ps < apparent {
		return ps
	}
	return apparent
}

// privateDataSize uses getattrlist(2) with ATTR_CMNEXT_PRIVATESIZE to query
// the number of bytes NOT shared with other files via APFS clone extents.
// On non-APFS volumes the syscall may fail; callers should fall back to
// apparent size.
func privateDataSize(path string) (int64, error) {
	p, err := unix.BytePtrFromString(path)
	if err != nil {
		return 0, err
	}
	attrList := unix.Attrlist{
		Bitmapcount: 5,
		Forkattr:    attrCmnextPrivatesize,
	}
	var buf [privateSizeBufLen]byte
	_, _, errno := unix.Syscall6(
		sysGetattrlist,
		uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&attrList)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(privateSizeBufLen),
		uintptr(unix.FSOPT_ATTR_CMN_EXTENDED),
		0,
	)
	if errno != 0 {
		return 0, errno
	}
	if binary.LittleEndian.Uint32(buf[:4]) < privateSizeBufLen {
		return 0, syscall.ENOTSUP
	}
	return int64(binary.LittleEndian.Uint64(buf[4:12])), nil
}
