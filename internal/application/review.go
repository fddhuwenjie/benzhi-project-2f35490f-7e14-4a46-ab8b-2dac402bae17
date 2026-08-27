package application

import (
	"context"
	"strings"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type SubmitReviewCommand struct {
	CommandMeta
	DrillID string `json:"-"`
}

func (s *Service) SubmitReview(ctx context.Context, command SubmitReviewCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "submit_review", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusReadyReview && a.Drill.Status != domain.StatusReturned {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		if !domain.AllInitialResults(a) || !domain.AllDeviationsClosed(a) {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		var roundNumber int
		if a.Drill.Status == domain.StatusReturned {
			round := domain.CurrentOpenReviewRound(a)
			if err := domain.FreezeReviewResponses(round, now); err != nil {
				return command.DrillID, 0, nil, err
			}
			roundNumber = round.Round
		}
		expected := a.Drill.Version
		a.Drill.Status = domain.StatusUnderReview
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "review_submitted", map[string]any{"version": a.Drill.Version, "round": roundNumber}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill}, nil
	})
}

type ReviewCommand struct {
	CommandMeta
	DrillID      string                   `json:"-"`
	Decision     string                   `json:"decision"`
	ReviewerName string                   `json:"reviewer_name"`
	ReviewNote   string                   `json:"review_note"`
	Items        []domain.ReviewItemInput `json:"items,omitempty"`
}

func (s *Service) Review(ctx context.Context, command ReviewCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "review_decision", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusUnderReview {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		if strings.TrimSpace(command.ReviewerName) == "" || strings.TrimSpace(command.ReviewerName) == strings.TrimSpace(a.Drill.LeadName) {
			problems := &domain.ValidationErrors{}
			problems.Add("reviewer_name", "独立复核员不得与演练负责人相同")
			return command.DrillID, 0, nil, problems
		}
		expected := a.Drill.Version
		if command.Decision == "rejected" {
			round, err := domain.BuildReviewRound(len(a.ReviewRounds)+1, command.ReviewerName, command.ReviewNote, command.Items, now, func() string { return newID("review_item") })
			if err != nil {
				return command.DrillID, 0, nil, err
			}
			a.ReviewRounds = append(a.ReviewRounds, round)
			a.Drill.Status = domain.StatusReturned
		} else if command.Decision == "approved" {
			decision, err := domain.BuildActivationDecision(a, newID("decision"), command.ReviewerName, command.ReviewNote, now)
			if err != nil {
				return command.DrillID, 0, nil, err
			}
			a.Decision = decision
			a.Drill.Status = domain.StatusActivated
		} else {
			return command.DrillID, 0, nil, domain.ErrValidation
		}
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		payload := map[string]any{"decision": command.Decision, "reviewer_name": strings.TrimSpace(command.ReviewerName), "review_note": strings.TrimSpace(command.ReviewNote)}
		if command.Decision == "rejected" {
			payload["round"] = len(a.ReviewRounds)
			payload["item_count"] = len(command.Items)
		}
		if a.Decision != nil {
			payload["document_digest"] = a.Decision.DocumentDigest
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "review_decided", payload, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "decision": a.Decision}, nil
	})
}

type ReviewResponsesCommand struct {
	CommandMeta
	DrillID   string                       `json:"-"`
	Responses []domain.ReviewResponseInput `json:"responses"`
}

func (s *Service) RespondToReview(ctx context.Context, command ReviewResponsesCommand) (CommandResult, error) {
	return s.execute(ctx, command.DrillID, "respond_review_items", command.CommandMeta, func(tx *storage.Tx, now time.Time) (string, int, any, error) {
		a, err := tx.LoadAggregate(ctx, command.DrillID)
		if err != nil {
			return command.DrillID, 0, nil, err
		}
		if err := checkVersion(a.Drill.Version, command.CommandMeta); err != nil {
			return command.DrillID, 0, nil, err
		}
		if a.Drill.Status != domain.StatusReturned {
			return command.DrillID, 0, nil, domain.ErrInvalidState
		}
		round := domain.CurrentOpenReviewRound(a)
		if err := domain.ApplyReviewResponses(round, command.Responses, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		expected := a.Drill.Version
		a.Drill.Version++
		a.Drill.UpdatedAt = now
		if err := tx.SaveAggregate(ctx, a, expected); err != nil {
			return command.DrillID, 0, nil, err
		}
		ids := make([]string, 0, len(command.Responses))
		for _, response := range command.Responses {
			ids = append(ids, response.ItemID)
		}
		if _, err := tx.AppendEvent(ctx, a.Drill.ID, "review_items_responded", map[string]any{"round": round.Round, "item_ids": ids}, now); err != nil {
			return command.DrillID, 0, nil, err
		}
		return command.DrillID, 0, map[string]any{"drill": a.Drill, "review_round": round}, nil
	})
}
