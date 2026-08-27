package domain

import (
	"fmt"
	"strings"
	"time"
)

type ResultInput struct {
	CheckpointCode   string    `json:"checkpoint_code"`
	ParticipantCount int       `json:"participant_count"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	Outcome          string    `json:"outcome"`
	EvidenceDigest   string    `json:"evidence_digest"`
	RecordedBy       string    `json:"recorded_by"`
}

type Evaluation struct {
	Passed   bool   `json:"passed"`
	RuleCode string `json:"rule_code,omitempty"`
	Message  string `json:"message"`
}

func ValidateResult(in ResultInput) error {
	var problems ValidationErrors
	if in.ParticipantCount <= 0 {
		problems.Add("participant_count", "参与人数必须大于零")
	}
	if in.StartedAt.IsZero() || in.EndedAt.IsZero() || !in.EndedAt.After(in.StartedAt) {
		problems.Add("ended_at", "结束时间必须晚于开始时间")
	}
	if in.Outcome != "pass" && in.Outcome != "fail" {
		problems.Add("outcome", "结论必须为 pass 或 fail")
	}
	if strings.TrimSpace(in.RecordedBy) == "" {
		problems.Add("recorded_by", "请填写记录人")
	}
	return problems.Err()
}

func ValidateResultForDrill(a *Aggregate, checkpoint Checkpoint, in ResultInput) error {
	var problems ValidationErrors
	if err := ValidateResult(in); err != nil {
		if validation, ok := err.(*ValidationErrors); ok {
			problems.Items = append(problems.Items, validation.Items...)
		}
	}
	if in.ParticipantCount > a.Drill.PlannedCapacity {
		problems.Add("participant_count", fmt.Sprintf("参与人数不能超过计划容量 %d", a.Drill.PlannedCapacity))
	}
	if !in.StartedAt.IsZero() && !in.EndedAt.IsZero() {
		location := in.StartedAt.Location()
		if in.StartedAt.In(location).Format("2006-01-02") != a.Drill.ScheduledDate {
			problems.Add("started_at", "开始时间必须位于演练日期当天")
		}
		if in.EndedAt.In(location).Format("2006-01-02") != a.Drill.ScheduledDate {
			problems.Add("ended_at", "结束时间必须位于演练日期当天")
		}
	}
	for _, preceding := range a.Checkpoints {
		if preceding.Order >= checkpoint.Order {
			break
		}
		for _, result := range a.Results {
			if result.CheckpointCode == preceding.Code && result.Attempt == 1 && in.StartedAt.Before(result.EndedAt) {
				problems.Add("started_at", fmt.Sprintf("开始时间不能早于前一检查点 %s 的结束时间", preceding.Name))
			}
		}
	}
	return problems.Err()
}

func EvaluateResult(cp Checkpoint, in ResultInput) Evaluation {
	if strings.TrimSpace(in.EvidenceDigest) == "" {
		return Evaluation{RuleCode: "evidence_missing", Message: "缺少证据摘要"}
	}
	seconds := int(in.EndedAt.Sub(in.StartedAt).Seconds())
	if seconds > cp.MaxSeconds {
		return Evaluation{RuleCode: "time_exceeded", Message: fmt.Sprintf("实测 %d 秒，超过 %d 秒上限", seconds, cp.MaxSeconds)}
	}
	if in.Outcome != "pass" {
		return Evaluation{RuleCode: "manual_failure", Message: "现场结论为不通过"}
	}
	return Evaluation{Passed: true, Message: "检查通过"}
}

func NextAttempt(a *Aggregate, code string) int {
	max := 0
	for _, result := range a.Results {
		if result.CheckpointCode == code && result.Attempt > max {
			max = result.Attempt
		}
	}
	return max + 1
}

func HasInitialResult(a *Aggregate, code string) bool {
	for _, result := range a.Results {
		if result.CheckpointCode == code && result.Attempt == 1 {
			return true
		}
	}
	return false
}

func AllInitialResults(a *Aggregate) bool {
	if len(a.Checkpoints) == 0 {
		return false
	}
	for _, cp := range a.Checkpoints {
		if !HasInitialResult(a, cp.Code) {
			return false
		}
	}
	return true
}

func AllDeviationsClosed(a *Aggregate) bool {
	for _, deviation := range a.Deviations {
		if deviation.Status != "closed" {
			return false
		}
	}
	return true
}
