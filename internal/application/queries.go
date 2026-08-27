package application

import (
	"context"
	"encoding/json"
	"time"

	"shelter-drill-gate/internal/audit"
	"shelter-drill-gate/internal/domain"
)

type TimelineItem struct {
	Sequence    int             `json:"sequence"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	OccurredAt  string          `json:"occurred_at"`
	CurrentHash string          `json:"current_hash"`
}

type Workbench struct {
	Aggregate        *domain.Aggregate        `json:"aggregate"`
	Timeline         []TimelineItem           `json:"timeline"`
	TimelineValid    bool                     `json:"timeline_valid"`
	DecisionValid    bool                     `json:"decision_valid"`
	DecisionDocument *domain.DecisionDocument `json:"decision_document,omitempty"`
	Progress         domain.ProgressSummary   `json:"progress"`
}

func (s *Service) ListDrills(ctx context.Context) ([]domain.Drill, error) {
	return s.store.ListDrills(ctx)
}

func (s *Service) GetWorkbench(ctx context.Context, id string) (*Workbench, error) {
	a, err := s.store.LoadAggregate(ctx, id)
	if err != nil {
		return nil, err
	}
	events, err := s.store.LoadEvents(ctx, id)
	if err != nil {
		return nil, err
	}
	view := &Workbench{Aggregate: a, TimelineValid: audit.Verify(events) == nil, Progress: domain.SummarizeProgress(a)}
	for _, event := range events {
		view.Timeline = append(view.Timeline, TimelineItem{Sequence: event.Sequence, EventType: event.EventType, Payload: json.RawMessage(event.Payload), OccurredAt: event.OccurredAt.Format(time.RFC3339), CurrentHash: event.CurrentHash})
	}
	if document, ok := domain.DecisionDocumentFor(a); ok {
		view.DecisionDocument = &document
		digest, err := audit.DigestDocument(document)
		view.DecisionValid = err == nil && digest == a.Decision.DocumentDigest
	}
	return view, nil
}

type DecisionVerification struct {
	Valid          bool                    `json:"valid"`
	DocumentDigest string                  `json:"document_digest"`
	Document       domain.DecisionDocument `json:"document"`
	Errors         []string                `json:"errors"`
}

func (s *Service) VerifyDecision(ctx context.Context, id string) (DecisionVerification, error) {
	a, err := s.store.LoadAggregate(ctx, id)
	if err != nil {
		return DecisionVerification{}, err
	}
	if a.Drill.Status != domain.StatusActivated || a.Decision == nil {
		return DecisionVerification{}, domain.ErrDecisionUnavailable
	}
	document, ok := domain.DecisionDocumentFor(a)
	if !ok {
		return DecisionVerification{}, domain.ErrDecisionUnavailable
	}
	verification := DecisionVerification{DocumentDigest: a.Decision.DocumentDigest, Document: document}
	if a.Decision.Decision != "approved" {
		verification.Errors = append(verification.Errors, "决定状态不是 approved")
	}
	if a.Decision.BaselineDigest == "" || a.Decision.BaselineDigest != a.Baseline.ContentDigest {
		verification.Errors = append(verification.Errors, "基线摘要不一致")
	}
	events, err := s.store.LoadEvents(ctx, id)
	if err != nil {
		return DecisionVerification{}, err
	}
	if err := audit.Verify(events); err != nil {
		verification.Errors = append(verification.Errors, "审计时间线不连续")
	}
	digest, err := audit.DigestDocument(document)
	if err != nil || digest != a.Decision.DocumentDigest {
		verification.Errors = append(verification.Errors, "决定书摘要不一致")
	}
	verification.Valid = len(verification.Errors) == 0
	return verification, nil
}
