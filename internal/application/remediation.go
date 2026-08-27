package application

import (
	"context"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type RemediateCommand struct {
	CommandMeta
	DrillID     string `json:"-"`
	DeviationID string `json:"-"`
	domain.RemediationInput
}

func (s *Service) Remediate(ctx context.Context, command RemediateCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "remediate_deviation", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusRemediation {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		deviation := deviationByID(a, command.DeviationID)
		if deviation == nil {
			return command.DrillID, 0, nil, domain.ErrNotFound
		}
		material, err := domain.BuildRemediationMaterial(deviation, newID("material"), command.RemediationInput, now)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		a.RemediationMaterials = append(a.RemediationMaterials, material)
		expected := a.Drill.Version
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "deviation_remediated", map[string]any{"deviation_id": deviation.ID, "checkpoint_code": deviation.CheckpointCode, "material_version": material.Version, "evidence_digest": deviation.EvidenceDigest}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "deviation": deviation, "material": material}, nil
	})
}

func deviationByID(a *domain.Aggregate, id string) *domain.Deviation {
	for i := range a.Deviations {
		if a.Deviations[i].ID == id {
			return &a.Deviations[i]
		}
	}
	return nil
}

type RetestCommand struct {
	CommandMeta
	DrillID     string `json:"-"`
	DeviationID string `json:"-"`
	domain.ResultInput
}

func (s *Service) Retest(ctx context.Context, command RetestCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "retest_deviation", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		deviation := deviationByID(a, command.DeviationID)
		if deviation == nil {
			return command.DrillID, 0, nil, domain.ErrNotFound
		}
		if deviation.Status != "ready_for_retest" || command.CheckpointCode != deviation.CheckpointCode {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		cp, ok := a.Checkpoint(deviation.CheckpointCode)
		if !ok {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		if err := domain.ValidateResultForDrill(a, cp, command.ResultInput); err != nil {
			return command.DrillID, 0, nil, err
		}
		evaluation := domain.EvaluateResult(cp, command.ResultInput)
		result := buildResult(a, command.ResultInput, domain.NextAttempt(a, cp.Code))
		a.Results = append(a.Results, result)
		attempt := domain.RetestAttempt{ID: newID("retest"), DeviationID: deviation.ID, ResultID: result.ID, MaterialVersion: deviation.MaterialVersion, Attempt: result.Attempt - 1, Passed: evaluation.Passed, RuleCode: evaluation.RuleCode, AttemptedAt: now.UTC()}
		if !evaluation.Passed {
			attempt.FailureReason = evaluation.Message
		}
		a.RetestAttempts = append(a.RetestAttempts, attempt)
		if evaluation.Passed {
			domain.CloseDeviation(deviation, result.ID, now)
		} else {
			deviation.Status = "ready_for_retest"
			deviation.RuleCode = evaluation.RuleCode
		}
		domain.RecalculateProgress(a)
		expected := a.Drill.Version
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "checkpoint_retested", map[string]any{"deviation_id": deviation.ID, "checkpoint_code": cp.Code, "result_id": result.ID, "attempt": result.Attempt, "material_version": attempt.MaterialVersion, "passed": evaluation.Passed, "rule_code": evaluation.RuleCode}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "result": result, "evaluation": evaluation, "deviation": deviation, "retest_attempt": attempt}, nil
	})
}
