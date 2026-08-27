package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type DecisionDocument struct {
	DocumentType    string `json:"document_type"`
	DrillID         string `json:"drill_id"`
	SiteName        string `json:"site_name"`
	ScheduledDate   string `json:"scheduled_date"`
	PlannedCapacity int    `json:"planned_capacity"`
	BaselineVersion int    `json:"baseline_version"`
	BaselineDigest  string `json:"baseline_digest"`
	Decision        string `json:"decision"`
	ReviewerName    string `json:"reviewer_name"`
	ReviewNote      string `json:"review_note"`
	IssuedAt        string `json:"issued_at"`
}

func BuildActivationDecision(a *Aggregate, id, reviewer, note string, now time.Time) (*ActivationDecision, error) {
	if a.Drill.Status != StatusUnderReview || !AllInitialResults(a) || !AllDeviationsClosed(a) {
		return nil, ErrInvalidState
	}
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" || reviewer == strings.TrimSpace(a.Drill.LeadName) {
		problems := &ValidationErrors{}
		problems.Add("reviewer_name", "独立复核员不得与演练负责人相同")
		return nil, problems
	}
	issued := now.UTC().Truncate(time.Second)
	doc := DecisionDocument{
		DocumentType: "shelter_activation_decision/v1", DrillID: a.Drill.ID,
		SiteName: a.Drill.SiteName, ScheduledDate: a.Drill.ScheduledDate,
		PlannedCapacity: a.Drill.PlannedCapacity, BaselineVersion: a.Baseline.Version,
		BaselineDigest: a.Baseline.ContentDigest, Decision: "approved",
		ReviewerName: reviewer, ReviewNote: strings.TrimSpace(note), IssuedAt: issued.Format(time.RFC3339),
	}
	raw, _ := json.Marshal(doc)
	hash := sha256.Sum256(raw)
	return &ActivationDecision{
		ID: id, DrillID: a.Drill.ID, Decision: "approved", ReviewerName: reviewer,
		ReviewNote: strings.TrimSpace(note), BaselineDigest: a.Baseline.ContentDigest,
		IssuedAt: issued, DocumentDigest: hex.EncodeToString(hash[:]),
	}, nil
}

func DecisionDocumentFor(a *Aggregate) (DecisionDocument, bool) {
	if a.Decision == nil {
		return DecisionDocument{}, false
	}
	d := a.Decision
	return DecisionDocument{
		DocumentType: "shelter_activation_decision/v1", DrillID: a.Drill.ID,
		SiteName: a.Drill.SiteName, ScheduledDate: a.Drill.ScheduledDate,
		PlannedCapacity: a.Drill.PlannedCapacity, BaselineVersion: a.Baseline.Version,
		BaselineDigest: d.BaselineDigest, Decision: d.Decision,
		ReviewerName: d.ReviewerName, ReviewNote: d.ReviewNote, IssuedAt: d.IssuedAt.UTC().Format(time.RFC3339),
	}, true
}
