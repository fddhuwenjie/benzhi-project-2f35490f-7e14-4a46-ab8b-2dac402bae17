package application

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

type drillResult struct {
	Drill      domain.Drill       `json:"drill"`
	Deviations []domain.Deviation `json:"deviations"`
	Deviation  *domain.Deviation  `json:"deviation"`
}

func decodeResult(t *testing.T, result CommandResult) drillResult {
	t.Helper()
	var value drillResult
	if err := json.Unmarshal(result.Body, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRemediationFlowIdempotencyAndVersionConflict(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store)
	clock := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	service.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	created, err := service.CreateDrill(ctx, CreateCommand{CommandMeta: CommandMeta{RequestID: "request-create", ExpectedVersion: 0}, CreateInput: domain.CreateInput{SiteName: "社区中心", PlannedCapacity: 200, LeadName: "负责人", ScheduledDate: "2026-08-27"}})
	if err != nil {
		t.Fatal(err)
	}
	current := decodeResult(t, created)
	id := current.Drill.ID
	frozen, err := service.FreezeBaseline(ctx, FreezeBaselineCommand{CommandMeta: CommandMeta{RequestID: "request-freeze", ExpectedVersion: current.Drill.Version}, DrillID: id, Entrances: []string{"东门"}, EvacuationRoutes: []string{"主通道"}, FunctionalZones: []string{"登记区"}, CriticalFacilities: []string{"应急灯"}})
	if err != nil {
		t.Fatal(err)
	}
	current = decodeResult(t, frozen)
	codes := []string{"entrance", "zone_guidance", "emergency_lighting", "supply_access"}
	start := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	for index, code := range codes {
		outcome := "pass"
		evidence := "evidence"
		if index == 0 {
			outcome = "fail"
		}
		result, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "request-check-" + code, ExpectedVersion: current.Drill.Version}, DrillID: id, ResultInput: domain.ResultInput{CheckpointCode: code, ParticipantCount: 50, StartedAt: start, EndedAt: start.Add(30 * time.Second), Outcome: outcome, EvidenceDigest: evidence, RecordedBy: "记录员"}})
		if err != nil {
			t.Fatalf("检查 %s: %v", code, err)
		}
		current = decodeResult(t, result)
		start = start.Add(time.Minute)
	}
	if current.Drill.Status != domain.StatusRemediation || len(current.Deviations) != 1 {
		t.Fatalf("偏差状态错误: %+v", current)
	}
	deviation := current.Deviations[0]
	remediated, err := service.Remediate(ctx, RemediateCommand{CommandMeta: CommandMeta{RequestID: "request-remediate", ExpectedVersion: current.Drill.Version}, DrillID: id, DeviationID: deviation.ID, RemediationInput: domain.RemediationInput{Cause: "入口堆物", CorrectiveAction: "清空通道", EvidenceDigest: "fix-evidence"}})
	if err != nil {
		t.Fatal(err)
	}
	current = decodeResult(t, remediated)
	retestCommand := RetestCommand{CommandMeta: CommandMeta{RequestID: "request-retest", ExpectedVersion: current.Drill.Version}, DrillID: id, DeviationID: deviation.ID, ResultInput: domain.ResultInput{CheckpointCode: "entrance", ParticipantCount: 50, StartedAt: start, EndedAt: start.Add(20 * time.Second), Outcome: "pass", EvidenceDigest: "retest-evidence", RecordedBy: "记录员"}}
	retested, err := service.Retest(ctx, retestCommand)
	if err != nil {
		t.Fatal(err)
	}
	after := decodeResult(t, retested)
	if after.Drill.Status != domain.StatusReadyReview {
		t.Fatalf("复验后状态: %s", after.Drill.Status)
	}
	replayed, err := service.Retest(ctx, retestCommand)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || string(replayed.Body) != string(retested.Body) {
		t.Fatal("重复 request_id 未返回原始结果")
	}
	view, err := service.GetWorkbench(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Aggregate.Results) != 5 || len(view.Aggregate.Deviations) != 1 {
		t.Fatalf("重复写入: results=%d deviations=%d", len(view.Aggregate.Results), len(view.Aggregate.Deviations))
	}
	_, err = service.SubmitReview(ctx, SubmitReviewCommand{CommandMeta: CommandMeta{RequestID: "request-stale", ExpectedVersion: 1}, DrillID: id})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("预期版本冲突，得到 %v", err)
	}
}
