// Package backupstatus reads the JSON status file written by the backup and
// restore-test scripts (WP-24) and consumed by /admin/health (S-10). The app
// only reads it — the scripts own writes. A missing file is not an error (the
// section is simply omitted); the admin UI never queries external systems.
package backupstatus

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/forecastiq/forecastiq/internal/admin"
)

// FileReader reads the backup status file at Path. An empty Path disables the
// section (Read returns nils).
type FileReader struct {
	Path string
}

// New returns a FileReader for the given path.
func New(path string) *FileReader { return &FileReader{Path: path} }

// entry is one status record in the file.
type entry struct {
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status"`
}

// file is the on-disk schema written by the backup/restore scripts.
type file struct {
	LastBackup      *entry `json:"last_backup"`
	LastRestoreTest *entry `json:"last_restore_test"`
}

// Read implements admin.BackupStatusReader. A missing file yields (nil, nil,
// nil) so the section is omitted rather than failing the health view.
func (r *FileReader) Read() (lastBackup, lastRestoreTest *admin.BackupStatus, err error) {
	if r.Path == "" {
		return nil, nil, nil
	}
	raw, rerr := os.ReadFile(r.Path)
	if rerr != nil {
		if os.IsNotExist(rerr) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read backup status: %w", rerr)
	}
	var f file
	if uerr := json.Unmarshal(raw, &f); uerr != nil {
		return nil, nil, fmt.Errorf("decode backup status: %w", uerr)
	}
	return toStatus(f.LastBackup), toStatus(f.LastRestoreTest), nil
}

func toStatus(e *entry) *admin.BackupStatus {
	if e == nil {
		return nil
	}
	return &admin.BackupStatus{CompletedAt: e.CompletedAt, Status: e.Status}
}
