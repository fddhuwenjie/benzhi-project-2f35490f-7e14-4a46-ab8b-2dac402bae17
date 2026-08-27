package application

import (
	"context"
	"net/http"
	"strings"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type CreateCommand struct {
	CommandMeta
	domain.CreateInput
}

func (s *Service) CreateDrill(ctx context.Context, command CreateCommand) (CommandResult, error) {
	return s.execute(ctx, "create:"+command.RequestID, "create_drill", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		input := domain.NormalizeCreate(command.CreateInput)
		if err := domain.ValidateCreate(input); err != nil {
			return "", 0, nil, err
		}
		drill := domain.Drill{
			ID: newID("drill"), SiteName: strings.TrimSpace(input.SiteName), PlannedCapacity: input.PlannedCapacity,
			LeadName: strings.TrimSpace(input.LeadName), ScheduledDate: input.ScheduledDate,
			Status: domain.StatusDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.InsertDrill(ctx, drill); err != nil {
			return "", 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, drill.ID, "drill_created", map[string]any{"site_name": drill.SiteName, "lead_name": drill.LeadName, "scheduled_date": drill.ScheduledDate}, now); err != nil {
			return "", 0, nil, err
		}
		return drill.ID, http.StatusCreated, map[string]any{"drill": drill}, nil
	})
}
