package application

import (
	"context"
	"fmt"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type RecordCheckCommand struct {
	CommandMeta
	DrillID string `json:"-"`
	domain.ResultInput
}

func (s *Service) RecordCheck(ctx context.Context, command RecordCheckCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "record_check", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusBaselineFrozen && a.Drill.Status != domain.StatusExecuting && a.Drill.Status != domain.StatusRemediation {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		cp, ok := a.Checkpoint(command.CheckpointCode)
		if !ok || domain.HasInitialResult(a, command.CheckpointCode) {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		for _, preceding := range a.Checkpoints {
			if preceding.Order >= cp.Order {
				break
			}
			if !domain.HasInitialResult(a, preceding.Code) {
				return command.DrillID, 0, nil, fmt.Errorf("请先完成检查点 %s: %w", preceding.Name, domain.ErrInvalidState)
			}
		}
		if err := domain.ValidateResultForDrill(a, cp, command.ResultInput); err != nil {
			return command.DrillID, 0, nil, err
		}
		evaluation := domain.EvaluateResult(cp, command.ResultInput)
		result := buildResult(a, command.ResultInput, 1)
		a.Results = append(a.Results, result)
		if !evaluation.Passed {
			a.Deviations = append(a.Deviations, domain.Deviation{ID: newID("dev"), DrillID: a.Drill.ID, CheckpointCode: cp.Code, RuleCode: evaluation.RuleCode, Status: "open"})
		}
		domain.RecalculateProgress(a)
		expected := a.Drill.Version
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "checkpoint_recorded", map[string]any{"checkpoint_code": cp.Code, "result_id": result.ID, "attempt": 1, "passed": evaluation.Passed, "rule_code": evaluation.RuleCode}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "result": result, "evaluation": evaluation, "deviations": a.Deviations}, nil
	})
}

func buildResult(a *domain.Aggregate, in domain.ResultInput, attempt int) domain.CheckpointResult {
	return domain.CheckpointResult{ID: newID("result"), DrillID: a.Drill.ID, CheckpointCode: in.CheckpointCode, Attempt: attempt, ParticipantCount: in.ParticipantCount, StartedAt: in.StartedAt.UTC(), EndedAt: in.EndedAt.UTC(), MeasuredSeconds: int(in.EndedAt.Sub(in.StartedAt).Seconds()), Outcome: in.Outcome, EvidenceDigest: in.EvidenceDigest, RecordedBy: in.RecordedBy}
}
