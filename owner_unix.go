//go:build unix

package markly

import (
	"os"
	"syscall"
)

// keepOwner gives f the owner and group of the file it replaces. Best
// effort: an unprivileged process can only keep its own uid and a group
// it belongs to, so a failure leaves the temp file's defaults.
func keepOwner(f *os.File, existing os.FileInfo) {
	st, ok := existing.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	if cur, err := f.Stat(); err == nil {
		if cst, ok := cur.Sys().(*syscall.Stat_t); ok && cst.Uid == st.Uid && cst.Gid == st.Gid {
			return
		}
	}
	_ = f.Chown(int(st.Uid), int(st.Gid))
}
