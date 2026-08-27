package domain

import (
	"strings"
	"time"
)

type RemediationInput struct {
	Cause            string `json:"cause"`
	CorrectiveAction string `json:"corrective_action"`
	EvidenceDigest   string `json:"evidence_digest"`
}

func ApplyRemediation(deviation *Deviation, in RemediationInput) error {
	if deviation.Status == "closed" {
		return ErrInvalidState
	}
	var problems ValidationErrors
	if strings.TrimSpace(in.Cause) == "" {
		problems.Add("cause", "请填写偏差原因")
	}
	if strings.TrimSpace(in.CorrectiveAction) == "" {
		problems.Add("corrective_action", "请填写纠正措施")
	}
	if strings.TrimSpace(in.EvidenceDigest) == "" {
		problems.Add("evidence_digest", "请填写纠正证据摘要")
	}
	if err := problems.Err(); err != nil {
		return err
	}
	deviation.Cause = strings.TrimSpace(in.Cause)
	deviation.CorrectiveAction = strings.TrimSpace(in.CorrectiveAction)
	deviation.EvidenceDigest = strings.TrimSpace(in.EvidenceDigest)
	deviation.Status = "ready_for_retest"
	return nil
}

func BuildRemediationMaterial(deviation *Deviation, id string, in RemediationInput, now time.Time) (RemediationMaterial, error) {
	if err := ApplyRemediation(deviation, in); err != nil {
		return RemediationMaterial{}, err
	}
	deviation.MaterialVersion++
	return RemediationMaterial{ID: id, DeviationID: deviation.ID, Version: deviation.MaterialVersion, Cause: deviation.Cause, CorrectiveAction: deviation.CorrectiveAction, EvidenceDigest: deviation.EvidenceDigest, SubmittedAt: now.UTC()}, nil
}

func CloseDeviation(deviation *Deviation, resultID string, now time.Time) {
	closedAt := now.UTC()
	deviation.Status = "closed"
	deviation.RetestResultID = resultID
	deviation.ClosedAt = &closedAt
}
