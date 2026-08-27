package domain

import "time"

type Drill struct {
	ID              string      `json:"id"`
	SiteName        string      `json:"site_name"`
	PlannedCapacity int         `json:"planned_capacity"`
	LeadName        string      `json:"lead_name"`
	ScheduledDate   string      `json:"scheduled_date"`
	Status          DrillStatus `json:"status"`
	BaselineVersion int         `json:"baseline_version"`
	Version         int         `json:"version"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type LayoutBaseline struct {
	DrillID            string    `json:"drill_id"`
	Version            int       `json:"version"`
	Entrances          []string  `json:"entrances"`
	EvacuationRoutes   []string  `json:"evacuation_routes"`
	FunctionalZones    []string  `json:"functional_zones"`
	CriticalFacilities []string  `json:"critical_facilities"`
	FrozenAt           time.Time `json:"frozen_at,omitempty"`
	ContentDigest      string    `json:"content_digest,omitempty"`
}

type Checkpoint struct {
	Code        string `json:"code"`
	Order       int    `json:"order"`
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
	MaxSeconds  int    `json:"max_seconds"`
}

type CheckpointResult struct {
	ID               string    `json:"id"`
	DrillID          string    `json:"drill_id"`
	CheckpointCode   string    `json:"checkpoint_code"`
	Attempt          int       `json:"attempt"`
	ParticipantCount int       `json:"participant_count"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	MeasuredSeconds  int       `json:"measured_seconds"`
	Outcome          string    `json:"outcome"`
	EvidenceDigest   string    `json:"evidence_digest"`
	RecordedBy       string    `json:"recorded_by"`
}

type Deviation struct {
	ID               string     `json:"id"`
	DrillID          string     `json:"drill_id"`
	CheckpointCode   string     `json:"checkpoint_code"`
	RuleCode         string     `json:"rule_code"`
	Status           string     `json:"status"`
	Cause            string     `json:"cause,omitempty"`
	CorrectiveAction string     `json:"corrective_action,omitempty"`
	EvidenceDigest   string     `json:"evidence_digest,omitempty"`
	RetestResultID   string     `json:"retest_result_id,omitempty"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	MaterialVersion  int        `json:"material_version"`
}

type RemediationMaterial struct {
	ID               string    `json:"id"`
	DeviationID      string    `json:"deviation_id"`
	Version          int       `json:"version"`
	Cause            string    `json:"cause"`
	CorrectiveAction string    `json:"corrective_action"`
	EvidenceDigest   string    `json:"evidence_digest"`
	SubmittedAt      time.Time `json:"submitted_at"`
}

type RetestAttempt struct {
	ID              string    `json:"id"`
	DeviationID     string    `json:"deviation_id"`
	ResultID        string    `json:"result_id"`
	MaterialVersion int       `json:"material_version"`
	Attempt         int       `json:"attempt"`
	Passed          bool      `json:"passed"`
	RuleCode        string    `json:"rule_code,omitempty"`
	FailureReason   string    `json:"failure_reason,omitempty"`
	AttemptedAt     time.Time `json:"attempted_at"`
}

type ReviewResponse struct {
	Response       string    `json:"response"`
	EvidenceDigest string    `json:"evidence_digest"`
	RespondedAt    time.Time `json:"responded_at"`
}

type ReviewItem struct {
	ID             string         `json:"id"`
	Round          int            `json:"round"`
	Description    string         `json:"description"`
	ReferenceType  string         `json:"reference_type"`
	ReferenceValue string         `json:"reference_value"`
	Requirement    string         `json:"requirement"`
	Response       ReviewResponse `json:"response"`
}

type ReviewRound struct {
	Round           int          `json:"round"`
	ReviewerName    string       `json:"reviewer_name"`
	ReviewNote      string       `json:"review_note"`
	ReturnedAt      time.Time    `json:"returned_at"`
	ResubmittedAt   *time.Time   `json:"resubmitted_at,omitempty"`
	ResponsesFrozen bool         `json:"responses_frozen"`
	Items           []ReviewItem `json:"items"`
}

type ActivationDecision struct {
	ID             string    `json:"id"`
	DrillID        string    `json:"drill_id"`
	Decision       string    `json:"decision"`
	ReviewerName   string    `json:"reviewer_name"`
	ReviewNote     string    `json:"review_note"`
	BaselineDigest string    `json:"baseline_digest"`
	IssuedAt       time.Time `json:"issued_at"`
	DocumentDigest string    `json:"document_digest"`
}

type Aggregate struct {
	Drill                Drill                 `json:"drill"`
	Baseline             LayoutBaseline        `json:"baseline"`
	Checkpoints          []Checkpoint          `json:"checkpoints"`
	Results              []CheckpointResult    `json:"results"`
	Deviations           []Deviation           `json:"deviations"`
	RemediationMaterials []RemediationMaterial `json:"remediation_materials"`
	RetestAttempts       []RetestAttempt       `json:"retest_attempts"`
	ReviewRounds         []ReviewRound         `json:"review_rounds"`
	Decision             *ActivationDecision   `json:"decision,omitempty"`
}

func (a *Aggregate) Checkpoint(code string) (Checkpoint, bool) {
	for _, cp := range a.Checkpoints {
		if cp.Code == code {
			return cp, true
		}
	}
	return Checkpoint{}, false
}

func (a *Aggregate) OpenDeviation(code string) (*Deviation, bool) {
	for i := range a.Deviations {
		if a.Deviations[i].CheckpointCode == code && a.Deviations[i].Status != "closed" {
			return &a.Deviations[i], true
		}
	}
	return nil, false
}
