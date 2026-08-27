package storage

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"shelter-drill-gate/internal/audit"
)

var auditStatementCache = struct {
	sync.Mutex
	statements map[string]*sql.Stmt
}{statements: make(map[string]*sql.Stmt)}

func (tx *Tx) AppendEvent(ctx context.Context, drillID, eventType string, payload any, now time.Time) (audit.Event, error) {
	var sequence int
	var previous string
	err := tx.tx.QueryRowContext(ctx, `SELECT sequence, current_hash FROM audit_events WHERE drill_id=? ORDER BY sequence DESC LIMIT 1`, drillID).Scan(&sequence, &previous)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return audit.Event{}, err
	}
	event, err := audit.BuildEvent(drillID, sequence+1, eventType, payload, now, previous)
	if err != nil {
		return audit.Event{}, err
	}
	_, err = tx.tx.ExecContext(ctx, `INSERT INTO audit_events(drill_id, sequence, event_type, payload, occurred_at, previous_hash, current_hash) VALUES(?,?,?,?,?,?,?)`,
		event.DrillID, event.Sequence, event.EventType, event.Payload, formatTime(event.OccurredAt), event.PreviousHash, event.CurrentHash)
	return event, err
}

func (s *Store) LoadEvents(ctx context.Context, drillID string) ([]audit.Event, error) {
	const query = `SELECT drill_id, sequence, event_type, payload, occurred_at, previous_hash, current_hash FROM audit_events WHERE drill_id=? ORDER BY sequence`
	auditStatementCache.Lock()
	statement := s.auditStatement
	if statement == nil {
		statement = auditStatementCache.statements[s.path]
		if statement == nil {
			var err error
			statement, err = s.db.PrepareContext(ctx, query)
			if err != nil {
				auditStatementCache.Unlock()
				return nil, err
			}
			auditStatementCache.statements[s.path] = statement
		}
		s.auditStatement = statement
	}
	auditStatementCache.Unlock()
	rows, err := statement.QueryContext(ctx, drillID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []audit.Event
	for rows.Next() {
		var event audit.Event
		var occurred string
		if err := rows.Scan(&event.DrillID, &event.Sequence, &event.EventType, &event.Payload, &occurred, &event.PreviousHash, &event.CurrentHash); err != nil {
			return nil, err
		}
		event.OccurredAt, _ = time.Parse(time.RFC3339Nano, occurred)
		events = append(events, event)
	}
	return events, rows.Err()
}
