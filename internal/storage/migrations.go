package storage

import (
	"context"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS drills (
		id TEXT PRIMARY KEY, site_name TEXT NOT NULL, planned_capacity INTEGER NOT NULL,
		lead_name TEXT NOT NULL, scheduled_date TEXT NOT NULL, status TEXT NOT NULL,
		baseline_version INTEGER NOT NULL, version INTEGER NOT NULL,
		created_at TEXT NOT NULL, updated_at TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS baselines (
		drill_id TEXT NOT NULL, version INTEGER NOT NULL, entrances TEXT NOT NULL,
		evacuation_routes TEXT NOT NULL, functional_zones TEXT NOT NULL,
		critical_facilities TEXT NOT NULL, frozen_at TEXT NOT NULL, content_digest TEXT NOT NULL,
		PRIMARY KEY (drill_id, version), FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS checkpoints (
		drill_id TEXT NOT NULL, code TEXT NOT NULL, ordering INTEGER NOT NULL,
		name TEXT NOT NULL, requirement TEXT NOT NULL, max_seconds INTEGER NOT NULL,
		PRIMARY KEY (drill_id, code), FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS checkpoint_results (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, checkpoint_code TEXT NOT NULL,
		attempt INTEGER NOT NULL, participant_count INTEGER NOT NULL,
		started_at TEXT NOT NULL, ended_at TEXT NOT NULL, measured_seconds INTEGER NOT NULL,
		outcome TEXT NOT NULL, evidence_digest TEXT NOT NULL, recorded_by TEXT NOT NULL,
		UNIQUE (drill_id, checkpoint_code, attempt), FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS deviations (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, checkpoint_code TEXT NOT NULL,
		rule_code TEXT NOT NULL, status TEXT NOT NULL, cause TEXT NOT NULL,
		corrective_action TEXT NOT NULL, evidence_digest TEXT NOT NULL,
		retest_result_id TEXT NOT NULL, closed_at TEXT NOT NULL,
		UNIQUE (drill_id, checkpoint_code), FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS activation_decisions (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL UNIQUE, decision TEXT NOT NULL,
		reviewer_name TEXT NOT NULL, review_note TEXT NOT NULL, baseline_digest TEXT NOT NULL,
		issued_at TEXT NOT NULL, document_digest TEXT NOT NULL UNIQUE,
		FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS audit_events (
		drill_id TEXT NOT NULL, sequence INTEGER NOT NULL, event_type TEXT NOT NULL,
		payload BLOB NOT NULL, occurred_at TEXT NOT NULL, previous_hash TEXT NOT NULL,
		current_hash TEXT NOT NULL UNIQUE, PRIMARY KEY (drill_id, sequence),
		FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS idempotency_results (
		request_id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, operation TEXT NOT NULL,
		http_status INTEGER NOT NULL, response BLOB NOT NULL, created_at TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_results_drill ON checkpoint_results(drill_id, checkpoint_code, attempt)`,
	`CREATE INDEX IF NOT EXISTS idx_deviations_drill ON deviations(drill_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_audit_drill ON audit_events(drill_id, sequence)`,
	`CREATE TABLE IF NOT EXISTS remediation_materials (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, deviation_id TEXT NOT NULL,
		version INTEGER NOT NULL, cause TEXT NOT NULL, corrective_action TEXT NOT NULL,
		evidence_digest TEXT NOT NULL, submitted_at TEXT NOT NULL,
		UNIQUE(deviation_id, version), FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS retest_attempts (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, deviation_id TEXT NOT NULL,
		result_id TEXT NOT NULL UNIQUE, material_version INTEGER NOT NULL, attempt INTEGER NOT NULL,
		passed INTEGER NOT NULL, rule_code TEXT NOT NULL, failure_reason TEXT NOT NULL,
		attempted_at TEXT NOT NULL, UNIQUE(deviation_id, attempt),
		FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS review_rounds (
		drill_id TEXT NOT NULL, round INTEGER NOT NULL, reviewer_name TEXT NOT NULL,
		review_note TEXT NOT NULL, returned_at TEXT NOT NULL, resubmitted_at TEXT NOT NULL,
		responses_frozen INTEGER NOT NULL, PRIMARY KEY(drill_id, round),
		FOREIGN KEY (drill_id) REFERENCES drills(id)
	)`,
	`CREATE TABLE IF NOT EXISTS review_items (
		id TEXT PRIMARY KEY, drill_id TEXT NOT NULL, round INTEGER NOT NULL,
		description TEXT NOT NULL, reference_type TEXT NOT NULL, reference_value TEXT NOT NULL,
		requirement TEXT NOT NULL, response TEXT NOT NULL, evidence_digest TEXT NOT NULL,
		responded_at TEXT NOT NULL, FOREIGN KEY (drill_id) REFERENCES drills(id),
		UNIQUE(drill_id, round, id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_materials_deviation ON remediation_materials(deviation_id, version)`,
	`CREATE INDEX IF NOT EXISTS idx_retests_deviation ON retest_attempts(deviation_id, attempt)`,
	`CREATE INDEX IF NOT EXISTS idx_review_items_round ON review_items(drill_id, round)`,
}

func (s *Store) Migrate(ctx context.Context) error {
	for index, statement := range migrations {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("执行数据库迁移 %d: %w", index+1, err)
		}
	}
	return nil
}
