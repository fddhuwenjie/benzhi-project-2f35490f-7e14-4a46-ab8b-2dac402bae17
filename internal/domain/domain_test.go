package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFreezeBaselineGeneratesOrderedCheckpoints(t *testing.T) {
	now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	a := &Aggregate{Drill: Drill{ID: "drill-1", Status: StatusDraft}, Baseline: LayoutBaseline{
		Entrances: []string{" 东门 ", "东门"}, EvacuationRoutes: []string{"主通道"},
		FunctionalZones: []string{"登记区"}, CriticalFacilities: []string{"应急灯"},
	}}
	if err := FreezeBaseline(a, now); err != nil {
		t.Fatalf("FreezeBaseline: %v", err)
	}
	if a.Drill.Status != StatusBaselineFrozen || a.Baseline.Version != 1 {
		t.Fatalf("冻结状态错误: %+v", a.Drill)
	}
	if len(a.Baseline.Entrances) != 1 {
		t.Fatalf("基线未去重: %#v", a.Baseline.Entrances)
	}
	if len(a.Checkpoints) != 4 || a.Checkpoints[0].Code != "entrance" || a.Checkpoints[3].Code != "supply_access" {
		t.Fatalf("检查点顺序错误: %#v", a.Checkpoints)
	}
	if len(a.Baseline.ContentDigest) != 64 {
		t.Fatalf("摘要长度错误: %s", a.Baseline.ContentDigest)
	}
}

func TestResultRules(t *testing.T) {
	cp := Checkpoint{Code: "entrance", MaxSeconds: 60}
	start := time.Now()
	cases := []struct {
		name  string
		input ResultInput
		rule  string
		pass  bool
	}{
		{"missing evidence", ResultInput{StartedAt: start, EndedAt: start.Add(10 * time.Second), Outcome: "pass"}, "evidence_missing", false},
		{"timeout", ResultInput{StartedAt: start, EndedAt: start.Add(61 * time.Second), Outcome: "pass", EvidenceDigest: "e"}, "time_exceeded", false},
		{"failed", ResultInput{StartedAt: start, EndedAt: start.Add(10 * time.Second), Outcome: "fail", EvidenceDigest: "e"}, "manual_failure", false},
		{"passed", ResultInput{StartedAt: start, EndedAt: start.Add(10 * time.Second), Outcome: "pass", EvidenceDigest: "e"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateResult(cp, tc.input)
			if got.RuleCode != tc.rule || got.Passed != tc.pass {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestDecisionRequiresIndependentReviewer(t *testing.T) {
	now := time.Now().UTC()
	a := &Aggregate{Drill: Drill{ID: "d", SiteName: "站点", LeadName: "负责人", Status: StatusUnderReview}, Baseline: LayoutBaseline{Version: 1, ContentDigest: "abc"}, Checkpoints: []Checkpoint{{Code: "entrance"}}, Results: []CheckpointResult{{CheckpointCode: "entrance", Attempt: 1}}}
	_, err := BuildActivationDecision(a, "decision", "负责人", "", now)
	var validation *ValidationErrors
	if !errors.As(err, &validation) {
		t.Fatalf("预期独立复核校验错误，得到 %v", err)
	}
	decision, err := BuildActivationDecision(a, "decision", "复核员", "同意", now)
	if err != nil || decision.DocumentDigest == "" {
		t.Fatalf("生成决定失败: %+v %v", decision, err)
	}
}
