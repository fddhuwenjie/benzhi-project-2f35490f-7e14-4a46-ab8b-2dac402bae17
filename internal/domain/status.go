package domain

type DrillStatus string

const (
	StatusDraft          DrillStatus = "draft"
	StatusBaselineFrozen DrillStatus = "baseline_frozen"
	StatusExecuting      DrillStatus = "executing"
	StatusRemediation    DrillStatus = "remediation"
	StatusReadyReview    DrillStatus = "ready_for_review"
	StatusUnderReview    DrillStatus = "under_review"
	StatusReturned       DrillStatus = "returned"
	StatusActivated      DrillStatus = "activated"
)

func (s DrillStatus) Terminal() bool { return s == StatusActivated }

func (s DrillStatus) Label() string {
	switch s {
	case StatusDraft:
		return "草稿"
	case StatusBaselineFrozen:
		return "基线已冻结"
	case StatusExecuting:
		return "检查执行中"
	case StatusRemediation:
		return "整改中"
	case StatusReadyReview:
		return "待送审"
	case StatusUnderReview:
		return "独立复核中"
	case StatusReturned:
		return "复核退回"
	case StatusActivated:
		return "已批准启用"
	default:
		return string(s)
	}
}
