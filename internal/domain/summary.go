package domain

import "sort"

type RuleSummary struct {
	RuleCode string `json:"rule_code"`
	Count    int    `json:"count"`
}

type OpenDeviationSummary struct {
	DeviationID    string `json:"deviation_id"`
	CheckpointCode string `json:"checkpoint_code"`
	CheckpointName string `json:"checkpoint_name"`
	RuleCode       string `json:"rule_code"`
	LatestAttempt  int    `json:"latest_attempt"`
	BlockingReason string `json:"blocking_reason"`
}

type ProgressSummary struct {
	InitialCompleted     int                    `json:"initial_completed"`
	Passed               int                    `json:"passed"`
	Failed               int                    `json:"failed"`
	OpenDeviations       int                    `json:"open_deviations"`
	ClosedDeviations     int                    `json:"closed_deviations"`
	CompletionPercent    int                    `json:"completion_percent"`
	RuleCounts           []RuleSummary          `json:"rule_counts"`
	OpenDeviationDetails []OpenDeviationSummary `json:"open_deviation_details"`
	NextCheckpoint       *Checkpoint            `json:"next_checkpoint,omitempty"`
	NextAction           string                 `json:"next_action"`
}

func SummarizeProgress(a *Aggregate) ProgressSummary {
	var summary ProgressSummary
	rules := make(map[string]int)
	for _, cp := range a.Checkpoints {
		var initial *CheckpointResult
		for i := range a.Results {
			if a.Results[i].CheckpointCode == cp.Code && a.Results[i].Attempt == 1 {
				initial = &a.Results[i]
				break
			}
		}
		if initial == nil {
			if summary.NextCheckpoint == nil {
				copy := cp
				summary.NextCheckpoint = &copy
			}
			continue
		}
		summary.InitialCompleted++
		if initialPassed(a, cp, *initial) {
			summary.Passed++
		} else {
			summary.Failed++
		}
	}
	if len(a.Checkpoints) > 0 {
		// 完成率体现已通过规则的比例，失败项在整改闭环后才计入完成。
		summary.CompletionPercent = summary.Passed * 100 / len(a.Checkpoints)
	}
	for _, deviation := range a.Deviations {
		rules[deviation.RuleCode]++
		if deviation.Status == "closed" {
			summary.ClosedDeviations++
			continue
		}
		summary.OpenDeviations++
		cp, _ := a.Checkpoint(deviation.CheckpointCode)
		summary.OpenDeviationDetails = append(summary.OpenDeviationDetails, OpenDeviationSummary{
			DeviationID: deviation.ID, CheckpointCode: deviation.CheckpointCode, CheckpointName: cp.Name,
			RuleCode: deviation.RuleCode, LatestAttempt: NextAttempt(a, deviation.CheckpointCode) - 1,
			BlockingReason: deviationBlockingReason(deviation),
		})
	}
	for code, count := range rules {
		summary.RuleCounts = append(summary.RuleCounts, RuleSummary{RuleCode: code, Count: count})
	}
	sort.Slice(summary.RuleCounts, func(i, j int) bool { return summary.RuleCounts[i].RuleCode < summary.RuleCounts[j].RuleCode })
	switch {
	case summary.NextCheckpoint != nil:
		summary.NextAction = "执行检查点：" + summary.NextCheckpoint.Name
	case summary.OpenDeviations > 0:
		summary.NextAction = "完成开放偏差的整改与定向复验"
	case a.Drill.Status == StatusReadyReview || a.Drill.Status == StatusReturned:
		summary.NextAction = "满足条件后提交独立复核"
	case a.Drill.Status == StatusUnderReview:
		summary.NextAction = "等待独立复核结论"
	case a.Drill.Status == StatusActivated:
		summary.NextAction = "核验并归档启用决定书"
	default:
		summary.NextAction = "按当前状态继续处理"
	}
	return summary
}

func initialPassed(a *Aggregate, cp Checkpoint, result CheckpointResult) bool {
	for _, deviation := range a.Deviations {
		if deviation.CheckpointCode == cp.Code {
			return deviation.Status == "closed"
		}
	}
	return result.Outcome == "pass" && result.EvidenceDigest != "" && result.MeasuredSeconds <= cp.MaxSeconds
}

func deviationBlockingReason(deviation Deviation) string {
	if deviation.Status == "ready_for_retest" {
		return "整改材料待定向复验"
	}
	switch deviation.RuleCode {
	case "evidence_missing":
		return "检查证据摘要缺失"
	case "time_exceeded":
		return "实测耗时超过规则上限"
	case "manual_failure":
		return "现场结论不通过"
	default:
		return "偏差尚未关闭"
	}
}
