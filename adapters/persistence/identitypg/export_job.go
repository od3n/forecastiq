package identitypg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/forecastiq/forecastiq/internal/identity/domain"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// ExportJobRepository implements ports.ExportJobRepository.
type ExportJobRepository struct{}

// NewExportJobRepository returns an ExportJobRepository.
func NewExportJobRepository() *ExportJobRepository { return &ExportJobRepository{} }

const exportColumns = `id, workspace_id, requested_by, target_user_id, status,
	object_key, expires_at, completed_at, error_message, created_at`

func scanExportJob(row pgx.Row) (*domain.ExportJob, error) {
	var j domain.ExportJob
	var objectKey, errMsg *string
	err := row.Scan(&j.ID, &j.WorkspaceID, &j.RequestedBy, &j.TargetUserID, &j.Status,
		&objectKey, &j.ExpiresAt, &j.CompletedAt, &errMsg, &j.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrExportNotFound
		}
		return nil, fmt.Errorf("scan export job: %w", err)
	}
	if objectKey != nil {
		j.ObjectKey = *objectKey
	}
	if errMsg != nil {
		j.ErrorMessage = *errMsg
	}
	return &j, nil
}

// Insert implements ports.ExportJobRepository. A unique violation on the
// one-active-per-user partial index becomes domain.ErrExportInProgress.
func (r *ExportJobRepository) Insert(ctx context.Context, tx dbtx.DBTX, j *domain.ExportJob) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO export_jobs (id, workspace_id, requested_by, target_user_id, status, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		j.ID, j.WorkspaceID, j.RequestedBy, j.TargetUserID, j.Status, j.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrExportInProgress
		}
		return fmt.Errorf("insert export job: %w", err)
	}
	return nil
}

// GetByID implements ports.ExportJobRepository.
func (r *ExportJobRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ExportJob, error) {
	return scanExportJob(tx.QueryRow(ctx, `SELECT `+exportColumns+` FROM export_jobs WHERE id = $1`, id))
}

// Complete implements ports.ExportJobRepository.
func (r *ExportJobRepository) Complete(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, objectKey string, expiresAt, completedAt time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE export_jobs SET status = 'completed', object_key = $2, expires_at = $3, completed_at = $4 WHERE id = $1`,
		id, objectKey, expiresAt, completedAt)
	if err != nil {
		return fmt.Errorf("complete export job: %w", err)
	}
	return nil
}

// Fail implements ports.ExportJobRepository.
func (r *ExportJobRepository) Fail(ctx context.Context, tx dbtx.DBTX, id uuid.UUID, msg string) error {
	_, err := tx.Exec(ctx,
		`UPDATE export_jobs SET status = 'failed', error_message = $2 WHERE id = $1`, id, msg)
	if err != nil {
		return fmt.Errorf("fail export job: %w", err)
	}
	return nil
}
