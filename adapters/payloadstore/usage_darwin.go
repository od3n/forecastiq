//go:build darwin

package payloadstore

import "syscall"

// diskUsage returns used/total bytes of the filesystem containing root. On
// Darwin the statfs block-count fields are already uint64; only Bsize (uint32)
// is converted.
func diskUsage(root string) (used, total uint64, err error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(root, &st); err != nil {
		return 0, 0, err
	}
	bsize := uint64(st.Bsize)
	total = st.Blocks * bsize
	free := st.Bavail * bsize
	if total > free {
		used = total - free
	}
	return used, total, nil
}
