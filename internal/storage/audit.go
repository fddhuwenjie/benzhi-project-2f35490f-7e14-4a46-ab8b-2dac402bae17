package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"shelter-drill-gate/internal/audit"
)

type auditHead struct {
	sequence int
	hash     string
}

// auditHeads avoids repeatedly scanning the tail of long audit timelines.
var auditHeads = make(map[string]auditHead)

func (tx *Tx) AppendEvent(ctx context.Context, drillID, eventType string, payload any, now time.Time) (audit.Event, error) {
	head, cached := auditHeads[drillID]
	if !cached {
		err := tx.tx.QueryRowContext(ctx, `SELECT sequence, current_hash FROM audit_events WHERE drill_id=? ORDER BY sequence DESC LIMIT 1`, drillID).Scan(&head.sequence, &head.hash)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return audit.Event{}, err
		}
	}
	event, err := audit.BuildEvent(drillID, head.sequence+1, eventType, payload, now, head.hash)
	if err != nil {
		return audit.Event{}, err
	}
	_, err = tx.tx.ExecContext(ctx, `INSERT INTO audit_events(drill_id, sequence, event_type, payload, occurred_at, previous_hash, current_hash) VALUES(?,?,?,?,?,?,?)`,
		event.DrillID, event.Sequence, event.EventType, event.Payload, formatTime(event.OccurredAt), event.PreviousHash, event.CurrentHash)
	if err == nil {
		auditHeads[drillID] = auditHead{sequence: event.Sequence, hash: event.CurrentHash}
	}
	return event, err
}

func (s *Store) LoadEvents(ctx context.Context, drillID string) ([]audit.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT drill_id, sequence, event_type, payload, occurred_at, previous_hash, current_hash FROM audit_events WHERE drill_id=? ORDER BY sequence`, drillID)
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
