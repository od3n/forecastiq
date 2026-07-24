package payloadstore

import (
	"fmt"
	"os"
	"syscall"
)

// VolumeUsage reports block-device usage of the payload volume (S-10 system
// section; NFR-OBS05). Bytes are computed from statfs of the store root.
type VolumeUsage struct {
	UsedBytes  uint64
	TotalBytes uint64
	UsedPct    float64
}

// Usage returns the payload volume's used/total bytes via statfs on the store
// root (created first so a fresh install still reports usage). Blocks and Bavail
// are uint64 on both Linux and Darwin; only Bsize (int64 Linux / uint32 Darwin)
// needs a conversion — so this compiles conversion-clean on both.
func (s *FilesystemStore) Usage() (VolumeUsage, error) {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return VolumeUsage{}, fmt.Errorf("payload volume: %w", err)
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(s.root, &st); err != nil {
		return VolumeUsage{}, fmt.Errorf("statfs payload volume: %w", err)
	}
	bsize := uint64(st.Bsize)
	total := st.Blocks * bsize
	free := st.Bavail * bsize
	used := uint64(0)
	if total > free {
		used = total - free
	}
	u := VolumeUsage{UsedBytes: used, TotalBytes: total}
	if total > 0 {
		u.UsedPct = float64(used) / float64(total) * 100
	}
	return u, nil
}
