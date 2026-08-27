package application

import (
	"context"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type ReviseDrillCommand struct {
	CommandMeta
	DrillID string `json:"-"`
	domain.CreateInput
}

func (s *Service) ReviseDrill(ctx context.Context, command ReviseDrillCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "revise_drill", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusDraft {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		input := domain.NormalizeCreate(command.CreateInput)
		if err := domain.ValidateCreate(input); err != nil {
			return command.DrillID, 0, nil, err
		}
		changed := make([]string, 0, 4)
		if a.Drill.SiteName != input.SiteName {
			changed = append(changed, "site_name")
		}
		if a.Drill.PlannedCapacity != input.PlannedCapacity {
			changed = append(changed, "planned_capacity")
		}
		if a.Drill.LeadName != input.LeadName {
			changed = append(changed, "lead_name")
		}
		if a.Drill.ScheduledDate != input.ScheduledDate {
			changed = append(changed, "scheduled_date")
		}
		expected := a.Drill.Version
		a.Drill.SiteName, a.Drill.PlannedCapacity = input.SiteName, input.PlannedCapacity
		a.Drill.LeadName, a.Drill.ScheduledDate = input.LeadName, input.ScheduledDate
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "drill_revised", map[string]any{"changed_fields": changed, "version": a.Drill.Version}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "changed_fields": changed}, nil
	})
}
