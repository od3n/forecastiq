//go:build linux

package payloadstore

import "syscall"

// diskUsage returns used/total bytes of the filesystem containing root. On
// Linux the statfs block fields are int64, so uint64 conversions are required.
func diskUsage(root string) (used, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return 0, 0, err
	}
	bsize := uint64(st.Bsize)
	total = uint64(st.Blocks) * bsize
	free := uint64(st.Bavail) * bsize
	if total > free {
		used = total - free
	}
	return used, total, nil
}
