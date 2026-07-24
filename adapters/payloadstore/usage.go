package payloadstore

import (
	"fmt"
	"os"
)

// VolumeUsage reports block-device usage of the payload volume (S-10 system
// section; NFR-OBS05). Bytes are computed from statfs of the store root.
type VolumeUsage struct {
	UsedBytes  uint64
	TotalBytes uint64
	UsedPct    float64
}

// Usage returns the payload volume's used/total bytes via statfs on the store
// root (created first so a fresh install still reports usage). The per-OS
// diskUsage helper keeps the statfs field-type differences (Linux vs Darwin)
// out of the shared code.
func (s *FilesystemStore) Usage() (VolumeUsage, error) {
	if err := os.MkdirAll(s.root, 0o750); err != nil {
		return VolumeUsage{}, fmt.Errorf("payload volume: %w", err)
	}
	used, total, err := diskUsage(s.root)
	if err != nil {
		return VolumeUsage{}, fmt.Errorf("statfs payload volume: %w", err)
	}
	u := VolumeUsage{UsedBytes: used, TotalBytes: total}
	if total > 0 {
		u.UsedPct = float64(used) / float64(total) * 100
	}
	return u, nil
}
