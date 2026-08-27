package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"shelter-drill-gate/internal/domain"
)

func (s *Store) LoadAggregate(ctx context.Context, id string) (*domain.Aggregate, error) {
	if cached, ok := s.aggregates.Load(id); ok {
		return cached.(*domain.Aggregate), nil
	}
	aggregate, err := loadAggregate(ctx, s.db, id)
	if err != nil {
		return nil, err
	}
	s.aggregates.Store(id, aggregate)
	return aggregate, nil
}

func (tx *Tx) LoadAggregate(ctx context.Context, id string) (*domain.Aggregate, error) {
	if cached, ok := tx.store.aggregates.Load(id); ok {
		return cached.(*domain.Aggregate), nil
	}
	aggregate, err := loadAggregate(ctx, tx.tx, id)
	if err != nil {
		return nil, err
	}
	tx.store.aggregates.Store(id, aggregate)
	return aggregate, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadAggregate(ctx context.Context, q queryer, id string) (*domain.Aggregate, error) {
	drill, err := scanDrill(q.QueryRowContext(ctx, `SELECT id, site_name, planned_capacity, lead_name, scheduled_date, status, baseline_version, version, created_at, updated_at FROM drills WHERE id=?`, id))
	if err != nil {
		return nil, err
	}
	a := &domain.Aggregate{Drill: drill}
	if err := loadBaseline(ctx, q, a); err != nil {
		return nil, err
	}
	if a.Checkpoints, err = loadCheckpoints(ctx, q, id); err != nil {
		return nil, err
	}
	if a.Results, err = loadResults(ctx, q, id); err != nil {
		return nil, err
	}
	if a.Deviations, err = loadDeviations(ctx, q, id); err != nil {
		return nil, err
	}
	if a.RemediationMaterials, err = loadRemediationMaterials(ctx, q, id); err != nil {
		return nil, err
	}
	for _, material := range a.RemediationMaterials {
		for index := range a.Deviations {
			if a.Deviations[index].ID == material.DeviationID && material.Version > a.Deviations[index].MaterialVersion {
				a.Deviations[index].MaterialVersion = material.Version
			}
		}
	}
	if a.RetestAttempts, err = loadRetestAttempts(ctx, q, id); err != nil {
		return nil, err
	}
	if a.ReviewRounds, err = loadReviewRounds(ctx, q, id); err != nil {
		return nil, err
	}
	if a.Decision, err = loadDecision(ctx, q, id); err != nil {
		return nil, err
	}
	return a, nil
}

func loadRemediationMaterials(ctx context.Context, q queryer, id string) ([]domain.RemediationMaterial, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, deviation_id, version, cause, corrective_action, evidence_digest, submitted_at FROM remediation_materials WHERE drill_id=? ORDER BY deviation_id, version`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.RemediationMaterial
	for rows.Next() {
		var value domain.RemediationMaterial
		var submitted string
		if err := rows.Scan(&value.ID, &value.DeviationID, &value.Version, &value.Cause, &value.CorrectiveAction, &value.EvidenceDigest, &submitted); err != nil {
			return nil, err
		}
		value.SubmittedAt, _ = time.Parse(time.RFC3339Nano, submitted)
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadRetestAttempts(ctx context.Context, q queryer, id string) ([]domain.RetestAttempt, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, deviation_id, result_id, material_version, attempt, passed, rule_code, failure_reason, attempted_at FROM retest_attempts WHERE drill_id=? ORDER BY deviation_id, attempt`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.RetestAttempt
	for rows.Next() {
		var value domain.RetestAttempt
		var attempted string
		if err := rows.Scan(&value.ID, &value.DeviationID, &value.ResultID, &value.MaterialVersion, &value.Attempt, &value.Passed, &value.RuleCode, &value.FailureReason, &attempted); err != nil {
			return nil, err
		}
		value.AttemptedAt, _ = time.Parse(time.RFC3339Nano, attempted)
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadReviewRounds(ctx context.Context, q queryer, id string) ([]domain.ReviewRound, error) {
	rows, err := q.QueryContext(ctx, `SELECT round, reviewer_name, review_note, returned_at, resubmitted_at, responses_frozen FROM review_rounds WHERE drill_id=? ORDER BY round`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rounds []domain.ReviewRound
	for rows.Next() {
		var round domain.ReviewRound
		var returned, resubmitted string
		if err := rows.Scan(&round.Round, &round.ReviewerName, &round.ReviewNote, &returned, &resubmitted, &round.ResponsesFrozen); err != nil {
			return nil, err
		}
		round.ReturnedAt, _ = time.Parse(time.RFC3339Nano, returned)
		if resubmitted != "" {
			parsed, _ := time.Parse(time.RFC3339Nano, resubmitted)
			round.ResubmittedAt = &parsed
		}
		round.Items, err = loadReviewItems(ctx, q, id, round.Round)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

func loadReviewItems(ctx context.Context, q queryer, id string, round int) ([]domain.ReviewItem, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, description, reference_type, reference_value, requirement, response, evidence_digest, responded_at FROM review_items WHERE drill_id=? AND round=? ORDER BY rowid`, id, round)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.ReviewItem
	for rows.Next() {
		var item domain.ReviewItem
		var responded string
		item.Round = round
		if err := rows.Scan(&item.ID, &item.Description, &item.ReferenceType, &item.ReferenceValue, &item.Requirement, &item.Response.Response, &item.Response.EvidenceDigest, &responded); err != nil {
			return nil, err
		}
		if responded != "" {
			item.Response.RespondedAt, _ = time.Parse(time.RFC3339Nano, responded)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadBaseline(ctx context.Context, q queryer, a *domain.Aggregate) error {
	if a.Drill.BaselineVersion == 0 {
		return nil
	}
	var entrances, routes, zones, facilities, frozen string
	err := q.QueryRowContext(ctx, `SELECT drill_id, version, entrances, evacuation_routes, functional_zones, critical_facilities, frozen_at, content_digest FROM baselines WHERE drill_id=? AND version=?`, a.Drill.ID, a.Drill.BaselineVersion).Scan(
		&a.Baseline.DrillID, &a.Baseline.Version, &entrances, &routes, &zones, &facilities, &frozen, &a.Baseline.ContentDigest)
	if err != nil {
		return err
	}
	_ = json.Unmarshal([]byte(entrances), &a.Baseline.Entrances)
	_ = json.Unmarshal([]byte(routes), &a.Baseline.EvacuationRoutes)
	_ = json.Unmarshal([]byte(zones), &a.Baseline.FunctionalZones)
	_ = json.Unmarshal([]byte(facilities), &a.Baseline.CriticalFacilities)
	a.Baseline.FrozenAt, _ = time.Parse(time.RFC3339Nano, frozen)
	return nil
}

func loadCheckpoints(ctx context.Context, q queryer, id string) ([]domain.Checkpoint, error) {
	rows, err := q.QueryContext(ctx, `SELECT code, ordering, name, requirement, max_seconds FROM checkpoints WHERE drill_id=? ORDER BY ordering`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.Checkpoint
	for rows.Next() {
		var value domain.Checkpoint
		if err := rows.Scan(&value.Code, &value.Order, &value.Name, &value.Requirement, &value.MaxSeconds); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadResults(ctx context.Context, q queryer, id string) ([]domain.CheckpointResult, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, drill_id, checkpoint_code, attempt, participant_count, started_at, ended_at, measured_seconds, outcome, evidence_digest, recorded_by FROM checkpoint_results WHERE drill_id=? ORDER BY checkpoint_code, attempt`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.CheckpointResult
	for rows.Next() {
		var value domain.CheckpointResult
		var started, ended string
		if err := rows.Scan(&value.ID, &value.DrillID, &value.CheckpointCode, &value.Attempt, &value.ParticipantCount, &started, &ended, &value.MeasuredSeconds, &value.Outcome, &value.EvidenceDigest, &value.RecordedBy); err != nil {
			return nil, err
		}
		value.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		value.EndedAt, _ = time.Parse(time.RFC3339Nano, ended)
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadDeviations(ctx context.Context, q queryer, id string) ([]domain.Deviation, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, drill_id, checkpoint_code, rule_code, status, cause, corrective_action, evidence_digest, retest_result_id, closed_at FROM deviations WHERE drill_id=? ORDER BY checkpoint_code`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []domain.Deviation
	for rows.Next() {
		var value domain.Deviation
		var closed string
		if err := rows.Scan(&value.ID, &value.DrillID, &value.CheckpointCode, &value.RuleCode, &value.Status, &value.Cause, &value.CorrectiveAction, &value.EvidenceDigest, &value.RetestResultID, &closed); err != nil {
			return nil, err
		}
		if closed != "" {
			parsed, _ := time.Parse(time.RFC3339Nano, closed)
			value.ClosedAt = &parsed
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func loadDecision(ctx context.Context, q queryer, id string) (*domain.ActivationDecision, error) {
	var value domain.ActivationDecision
	var issued string
	err := q.QueryRowContext(ctx, `SELECT id, drill_id, decision, reviewer_name, review_note, baseline_digest, issued_at, document_digest FROM activation_decisions WHERE drill_id=?`, id).Scan(
		&value.ID, &value.DrillID, &value.Decision, &value.ReviewerName, &value.ReviewNote, &value.BaselineDigest, &issued, &value.DocumentDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	value.IssuedAt, _ = time.Parse(time.RFC3339Nano, issued)
	return &value, nil
}
