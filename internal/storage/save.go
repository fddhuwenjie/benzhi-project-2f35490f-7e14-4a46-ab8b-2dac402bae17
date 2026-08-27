package storage

import (
	"context"
	"encoding/json"

	"shelter-drill-gate/internal/domain"
)

func (tx *Tx) SaveAggregate(ctx context.Context, a *domain.Aggregate, expectedVersion int) error {
	if err := tx.UpdateDrill(ctx, a.Drill, expectedVersion); err != nil {
		return err
	}
	if a.Baseline.Version > 0 {
		if err := tx.saveBaseline(ctx, a.Baseline); err != nil {
			return err
		}
	}
	for _, cp := range a.Checkpoints {
		if _, err := tx.tx.ExecContext(ctx, `INSERT INTO checkpoints(drill_id, code, ordering, name, requirement, max_seconds) VALUES(?,?,?,?,?,?) ON CONFLICT(drill_id,code) DO UPDATE SET ordering=excluded.ordering,name=excluded.name,requirement=excluded.requirement,max_seconds=excluded.max_seconds`, a.Drill.ID, cp.Code, cp.Order, cp.Name, cp.Requirement, cp.MaxSeconds); err != nil {
			return err
		}
	}
	for _, result := range a.Results {
		if _, err := tx.tx.ExecContext(ctx, `INSERT OR IGNORE INTO checkpoint_results(id, drill_id, checkpoint_code, attempt, participant_count, started_at, ended_at, measured_seconds, outcome, evidence_digest, recorded_by) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			result.ID, result.DrillID, result.CheckpointCode, result.Attempt, result.ParticipantCount, formatTime(result.StartedAt), formatTime(result.EndedAt), result.MeasuredSeconds, result.Outcome, result.EvidenceDigest, result.RecordedBy); err != nil {
			return err
		}
	}
	for _, deviation := range a.Deviations {
		closed := ""
		if deviation.ClosedAt != nil {
			closed = formatTime(*deviation.ClosedAt)
		}
		if _, err := tx.tx.ExecContext(ctx, `INSERT INTO deviations(id, drill_id, checkpoint_code, rule_code, status, cause, corrective_action, evidence_digest, retest_result_id, closed_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(drill_id,checkpoint_code) DO UPDATE SET rule_code=excluded.rule_code,status=excluded.status,cause=excluded.cause,corrective_action=excluded.corrective_action,evidence_digest=excluded.evidence_digest,retest_result_id=excluded.retest_result_id,closed_at=excluded.closed_at`,
			deviation.ID, deviation.DrillID, deviation.CheckpointCode, deviation.RuleCode, deviation.Status, deviation.Cause, deviation.CorrectiveAction, deviation.EvidenceDigest, deviation.RetestResultID, closed); err != nil {
			return err
		}
	}
	for _, material := range a.RemediationMaterials {
		if _, err := tx.tx.ExecContext(ctx, `INSERT OR IGNORE INTO remediation_materials(id, drill_id, deviation_id, version, cause, corrective_action, evidence_digest, submitted_at) VALUES(?,?,?,?,?,?,?,?)`,
			material.ID, a.Drill.ID, material.DeviationID, material.Version, material.Cause, material.CorrectiveAction, material.EvidenceDigest, formatTime(material.SubmittedAt)); err != nil {
			return err
		}
	}
	for _, attempt := range a.RetestAttempts {
		if _, err := tx.tx.ExecContext(ctx, `INSERT OR IGNORE INTO retest_attempts(id, drill_id, deviation_id, result_id, material_version, attempt, passed, rule_code, failure_reason, attempted_at) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			attempt.ID, a.Drill.ID, attempt.DeviationID, attempt.ResultID, attempt.MaterialVersion, attempt.Attempt, attempt.Passed, attempt.RuleCode, attempt.FailureReason, formatTime(attempt.AttemptedAt)); err != nil {
			return err
		}
	}
	for _, round := range a.ReviewRounds {
		resubmittedAt := ""
		if round.ResubmittedAt != nil {
			resubmittedAt = formatTime(*round.ResubmittedAt)
		}
		if _, err := tx.tx.ExecContext(ctx, `INSERT INTO review_rounds(drill_id, round, reviewer_name, review_note, returned_at, resubmitted_at, responses_frozen) VALUES(?,?,?,?,?,?,?) ON CONFLICT(drill_id,round) DO UPDATE SET resubmitted_at=excluded.resubmitted_at,responses_frozen=excluded.responses_frozen`,
			a.Drill.ID, round.Round, round.ReviewerName, round.ReviewNote, formatTime(round.ReturnedAt), resubmittedAt, round.ResponsesFrozen); err != nil {
			return err
		}
		for _, item := range round.Items {
			respondedAt := ""
			if !item.Response.RespondedAt.IsZero() {
				respondedAt = formatTime(item.Response.RespondedAt)
			}
			if _, err := tx.tx.ExecContext(ctx, `INSERT INTO review_items(id, drill_id, round, description, reference_type, reference_value, requirement, response, evidence_digest, responded_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET response=excluded.response,evidence_digest=excluded.evidence_digest,responded_at=excluded.responded_at`,
				item.ID, a.Drill.ID, round.Round, item.Description, item.ReferenceType, item.ReferenceValue, item.Requirement, item.Response.Response, item.Response.EvidenceDigest, respondedAt); err != nil {
				return err
			}
		}
	}
	if a.Decision != nil {
		d := a.Decision
		if _, err := tx.tx.ExecContext(ctx, `INSERT OR IGNORE INTO activation_decisions(id, drill_id, decision, reviewer_name, review_note, baseline_digest, issued_at, document_digest) VALUES(?,?,?,?,?,?,?,?)`, d.ID, d.DrillID, d.Decision, d.ReviewerName, d.ReviewNote, d.BaselineDigest, formatTime(d.IssuedAt), d.DocumentDigest); err != nil {
			return err
		}
	}
	return nil
}

func (tx *Tx) saveBaseline(ctx context.Context, b domain.LayoutBaseline) error {
	entrances, _ := json.Marshal(b.Entrances)
	routes, _ := json.Marshal(b.EvacuationRoutes)
	zones, _ := json.Marshal(b.FunctionalZones)
	facilities, _ := json.Marshal(b.CriticalFacilities)
	_, err := tx.tx.ExecContext(ctx, `INSERT OR IGNORE INTO baselines(drill_id, version, entrances, evacuation_routes, functional_zones, critical_facilities, frozen_at, content_digest) VALUES(?,?,?,?,?,?,?,?)`,
		b.DrillID, b.Version, entrances, routes, zones, facilities, formatTime(b.FrozenAt), b.ContentDigest)
	return err
}
