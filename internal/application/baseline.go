package application

import (
	"context"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type FreezeBaselineCommand struct {
	CommandMeta
	DrillID            string   `json:"-"`
	Entrances          []string `json:"entrances"`
	EvacuationRoutes   []string `json:"evacuation_routes"`
	FunctionalZones    []string `json:"functional_zones"`
	CriticalFacilities []string `json:"critical_facilities"`
	PreviewDigest      string   `json:"preview_digest,omitempty"`
}

type BaselinePreviewCommand struct {
	Entrances          []string `json:"entrances"`
	EvacuationRoutes   []string `json:"evacuation_routes"`
	FunctionalZones    []string `json:"functional_zones"`
	CriticalFacilities []string `json:"critical_facilities"`
}

func (s *Service) PreviewBaseline(ctx context.Context, drillID string, command BaselinePreviewCommand) (domain.BaselinePreview, error) {
	a, err := s.store.LoadAggregate(ctx, drillID)
	if err != nil {
		return domain.BaselinePreview{}, err
	}
	if a.Drill.Status != domain.StatusDraft {
		return domain.BaselinePreview{}, domain.ErrInvalidState
	}
	return domain.PreviewBaseline(a, domain.LayoutBaseline{DrillID: drillID, Entrances: command.Entrances, EvacuationRoutes: command.EvacuationRoutes, FunctionalZones: command.FunctionalZones, CriticalFacilities: command.CriticalFacilities}), nil
}

func (s *Service) FreezeBaseline(ctx context.Context, command FreezeBaselineCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "freeze_baseline", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		a.Baseline = domain.LayoutBaseline{DrillID: command.DrillID, Entrances: command.Entrances, EvacuationRoutes: command.EvacuationRoutes, FunctionalZones: command.FunctionalZones, CriticalFacilities: command.CriticalFacilities}
		preview := domain.PreviewBaseline(a, a.Baseline)
		if command.PreviewDigest != "" && command.PreviewDigest != preview.PreviewDigest {
			return command.DrillID, 0, nil, domain.ErrConflict
		}
		if err := domain.FreezeBaseline(a, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		expected := a.Drill.Version
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "baseline_frozen", map[string]any{"baseline_version": a.Baseline.Version, "content_digest": a.Baseline.ContentDigest, "checkpoint_count": len(a.Checkpoints)}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "baseline": a.Baseline, "checkpoints": a.Checkpoints}, nil
	})
}
