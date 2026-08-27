package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"shelter-drill-gate/internal/domain"
)

func (tx *Tx) InsertDrill(ctx context.Context, drill domain.Drill) error {
	_, err := tx.tx.ExecContext(ctx, `INSERT INTO drills(id, site_name, planned_capacity, lead_name, scheduled_date, status, baseline_version, version, created_at, updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		drill.ID, drill.SiteName, drill.PlannedCapacity, drill.LeadName, drill.ScheduledDate, drill.Status,
		drill.BaselineVersion, drill.Version, formatTime(drill.CreatedAt), formatTime(drill.UpdatedAt))
	return err
}

func (tx *Tx) UpdateDrill(ctx context.Context, drill domain.Drill, expectedVersion int) error {
	result, err := tx.tx.ExecContext(ctx, `UPDATE drills SET site_name=?, planned_capacity=?, lead_name=?, scheduled_date=?, status=?, baseline_version=?, version=?, updated_at=? WHERE id=? AND version=?`,
		drill.SiteName, drill.PlannedCapacity, drill.LeadName, drill.ScheduledDate, drill.Status,
		drill.BaselineVersion, drill.Version, formatTime(drill.UpdatedAt), drill.ID, expectedVersion)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return domain.ErrConflict
	}
	return nil
}

func (s *Store) ListDrills(ctx context.Context) ([]domain.Drill, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, site_name, planned_capacity, lead_name, scheduled_date, status, baseline_version, version, created_at, updated_at FROM drills ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drills []domain.Drill
	for rows.Next() {
		drill, err := scanDrill(rows)
		if err != nil {
			return nil, err
		}
		drills = append(drills, drill)
	}
	return drills, rows.Err()
}

func scanDrill(row rowScanner) (domain.Drill, error) {
	var drill domain.Drill
	var created, updated string
	err := row.Scan(&drill.ID, &drill.SiteName, &drill.PlannedCapacity, &drill.LeadName,
		&drill.ScheduledDate, &drill.Status, &drill.BaselineVersion, &drill.Version, &created, &updated)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return drill, domain.ErrNotFound
		}
		return drill, err
	}
	drill.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	drill.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updated)
	return drill, nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }
