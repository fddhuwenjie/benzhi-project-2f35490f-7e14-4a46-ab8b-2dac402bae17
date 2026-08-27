package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type StoredResponse struct {
	RequestID string
	DrillID   string
	Operation string
	Status    int
	Body      []byte
	CreatedAt time.Time
}

func (s *Store) FindResponse(ctx context.Context, requestID string) (*StoredResponse, error) {
	row := s.db.QueryRowContext(ctx, `SELECT request_id, drill_id, operation, http_status, response, created_at FROM idempotency_results WHERE request_id = ?`, requestID)
	return scanStoredResponse(row)
}

func (tx *Tx) FindResponse(ctx context.Context, requestID string) (*StoredResponse, error) {
	row := tx.tx.QueryRowContext(ctx, `SELECT request_id, drill_id, operation, http_status, response, created_at FROM idempotency_results WHERE request_id = ?`, requestID)
	return scanStoredResponse(row)
}

type rowScanner interface{ Scan(...any) error }

func scanStoredResponse(row rowScanner) (*StoredResponse, error) {
	var result StoredResponse
	var created string
	if err := row.Scan(&result.RequestID, &result.DrillID, &result.Operation, &result.Status, &result.Body, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	result.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &result, nil
}

func (tx *Tx) SaveResponse(ctx context.Context, result StoredResponse) error {
	_, err := tx.tx.ExecContext(ctx, `INSERT INTO idempotency_results(request_id, drill_id, operation, http_status, response, created_at) VALUES(?,?,?,?,?,?)`,
		result.RequestID, result.DrillID, result.Operation, result.Status, result.Body, result.CreatedAt.UTC().Format(time.RFC3339Nano))
	return err
}
