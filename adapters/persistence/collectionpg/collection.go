// Package collectionpg implements the collection module's repository ports on
// PostgreSQL (pgx), including the partitioned forecast_snapshots batch insert
// (ON CONFLICT DO NOTHING dedup) and runtime partition management. Wired only
// in cmd/ (composition root); parameterized queries throughout.
package collectionpg

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/forecastiq/forecastiq/internal/collection/domain"
	"github.com/forecastiq/forecastiq/internal/collection/ports"
	"github.com/forecastiq/forecastiq/internal/platform/dbtx"
)

// isUniqueViolation reports whether err is a PostgreSQL unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CollectionRepository implements ports.CollectionRepository.
type CollectionRepository struct{}

// NewCollectionRepository returns a CollectionRepository.
func NewCollectionRepository() *CollectionRepository { return &CollectionRepository{} }

// collectionSelect lists collection columns with COALESCE for nullable
// text/int fields so scans target non-pointer Go types.
const collectionSelect = `id, provider_id, location_id, provider_configuration_id,
	requested_at, completed_at, collection_status,
	COALESCE(provider_request_id,''), provider_model_run_time,
	COALESCE(raw_payload_object_key,''), COALESCE(raw_payload_checksum,''),
	COALESCE(response_status_code,0), COALESCE(response_latency_ms,0),
	records_received, snapshots_stored, snapshots_deduplicated, snapshots_invalid,
	COALESCE(schema_version,''), COALESCE(adapter_version,''),
	COALESCE(error_code,''), COALESCE(error_message,''), created_at`

func scanCollection(row pgx.Row) (*domain.ForecastCollection, error) {
	var c domain.ForecastCollection
	var status string
	err := row.Scan(&c.ID, &c.ProviderID, &c.LocationID, &c.ProviderConfigurationID,
		&c.RequestedAt, &c.CompletedAt, &status,
		&c.ProviderRequestID, &c.ProviderModelRunTime,
		&c.RawPayloadObjectKey, &c.RawPayloadChecksum,
		&c.ResponseStatusCode, &c.ResponseLatencyMS,
		&c.RecordsReceived, &c.SnapshotsStored, &c.SnapshotsDeduplicated, &c.SnapshotsInvalid,
		&c.SchemaVersion, &c.AdapterVersion, &c.ErrorCode, &c.ErrorMessage, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("scan collection: %w", err)
	}
	c.Status = domain.CollectionStatus(status)
	return &c, nil
}

// Insert implements ports.CollectionRepository.
func (r *CollectionRepository) Insert(ctx context.Context, tx dbtx.DBTX, c *domain.ForecastCollection) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO forecast_collections (
		   id, provider_id, location_id, provider_configuration_id, requested_at, completed_at,
		   collection_status, provider_request_id, provider_model_run_time,
		   raw_payload_object_key, raw_payload_checksum, response_status_code, response_latency_ms,
		   records_received, snapshots_stored, snapshots_deduplicated, snapshots_invalid,
		   schema_version, adapter_version, error_code, error_message, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22)`,
		c.ID, c.ProviderID, c.LocationID, c.ProviderConfigurationID, c.RequestedAt, c.CompletedAt,
		string(c.Status), c.ProviderRequestID, c.ProviderModelRunTime,
		c.RawPayloadObjectKey, c.RawPayloadChecksum, c.ResponseStatusCode, c.ResponseLatencyMS,
		c.RecordsReceived, c.SnapshotsStored, c.SnapshotsDeduplicated, c.SnapshotsInvalid,
		c.SchemaVersion, c.AdapterVersion, c.ErrorCode, c.ErrorMessage, c.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert collection: %w", err)
	}
	return nil
}

// Complete implements ports.CollectionRepository (single final UPDATE; allowed
// only while the row is pending — immutability trigger enforces thereafter).
func (r *CollectionRepository) Complete(ctx context.Context, tx dbtx.DBTX, c *domain.ForecastCollection) error {
	_, err := tx.Exec(ctx,
		`UPDATE forecast_collections SET
		   completed_at = $2, collection_status = $3,
		   records_received = $4, snapshots_stored = $5, snapshots_deduplicated = $6, snapshots_invalid = $7,
		   error_code = $8, error_message = $9
		 WHERE id = $1`,
		c.ID, c.CompletedAt, string(c.Status),
		c.RecordsReceived, c.SnapshotsStored, c.SnapshotsDeduplicated, c.SnapshotsInvalid,
		c.ErrorCode, c.ErrorMessage)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateCollection
		}
		return fmt.Errorf("complete collection: %w", err)
	}
	return nil
}

// GetByID implements ports.CollectionRepository.
func (r *CollectionRepository) GetByID(ctx context.Context, tx dbtx.DBTX, id uuid.UUID) (*domain.ForecastCollection, error) {
	return scanCollection(tx.QueryRow(ctx, `SELECT `+collectionSelect+` FROM forecast_collections WHERE id = $1`, id))
}

// FindDedup implements ports.CollectionRepository (domain §4.3). Returns nil
// when no matching successful collection exists.
func (r *CollectionRepository) FindDedup(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID, dedupKey time.Time) (*domain.ForecastCollection, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+collectionSelect+` FROM forecast_collections
		 WHERE provider_id = $1 AND location_id = $2
		   AND COALESCE(provider_model_run_time, requested_at) = $3
		   AND collection_status IN ('success','partial')
		 LIMIT 1`, providerID, locationID, dedupKey)
	c, err := scanCollection(row)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return c, err
}

// LatestSuccessful implements ports.CollectionRepository. Returns nil when none.
func (r *CollectionRepository) LatestSuccessful(ctx context.Context, tx dbtx.DBTX, providerID, locationID uuid.UUID) (*domain.ForecastCollection, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+collectionSelect+` FROM forecast_collections
		 WHERE provider_id = $1 AND location_id = $2 AND collection_status IN ('success','partial')
		 ORDER BY requested_at DESC LIMIT 1`, providerID, locationID)
	c, err := scanCollection(row)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	return c, err
}

// List implements ports.CollectionRepository (keyset pagination by id).
func (r *CollectionRepository) List(ctx context.Context, tx dbtx.DBTX, f ports.CollectionFilter) ([]*domain.ForecastCollection, error) {
	query := `SELECT ` + collectionSelect + ` FROM forecast_collections WHERE id > $1`
	args := []any{f.Cursor}
	idx := 2
	if f.ProviderID != nil {
		query += fmt.Sprintf(` AND provider_id = $%d`, idx)
		args = append(args, *f.ProviderID)
		idx++
	}
	if f.LocationID != nil {
		query += fmt.Sprintf(` AND location_id = $%d`, idx)
		args = append(args, *f.LocationID)
		idx++
	}
	if f.Status != nil {
		query += fmt.Sprintf(` AND collection_status = $%d`, idx)
		args = append(args, string(*f.Status))
		idx++
	}
	query += fmt.Sprintf(` ORDER BY id ASC LIMIT $%d`, idx)
	args = append(args, f.Limit+1)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}
	defer rows.Close()
	var out []*domain.ForecastCollection
	for rows.Next() {
		c, err := scanCollection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ProviderLineages implements ports.CollectionRepository. Per provider it
// returns the earliest collection requested_at (collecting_since) and the
// adapter_version of the most recent successful/partial collection (latest
// non-empty wins). Providers with no collections are simply absent.
func (r *CollectionRepository) ProviderLineages(ctx context.Context, tx dbtx.DBTX) ([]ports.ProviderLineage, error) {
	rows, err := tx.Query(ctx,
		`SELECT provider_id,
		        min(requested_at) AS collecting_since,
		        COALESCE((array_agg(adapter_version ORDER BY requested_at DESC)
		          FILTER (WHERE collection_status IN ('success','partial')
		            AND adapter_version IS NOT NULL AND adapter_version <> ''))[1], '') AS adapter_version
		 FROM forecast_collections
		 GROUP BY provider_id`)
	if err != nil {
		return nil, fmt.Errorf("provider lineages: %w", err)
	}
	defer rows.Close()
	var out []ports.ProviderLineage
	for rows.Next() {
		var l ports.ProviderLineage
		if err := rows.Scan(&l.ProviderID, &l.CollectingSince, &l.AdapterVersion); err != nil {
			return nil, fmt.Errorf("scan provider lineage: %w", err)
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
