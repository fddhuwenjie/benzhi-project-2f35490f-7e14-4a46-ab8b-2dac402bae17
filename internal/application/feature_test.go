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

func featureService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "features.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := NewService(store)
	clock := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	return service, context.Background()
}

func commandDrill(t *testing.T, result CommandResult) domain.Drill {
	t.Helper()
	var body struct {
		Drill domain.Drill `json:"drill"`
	}
	if err := json.Unmarshal(result.Body, &body); err != nil {
		t.Fatal(err)
	}
	return body.Drill
}

func createFeatureDrill(t *testing.T, service *Service, ctx context.Context, capacity int) domain.Drill {
	t.Helper()
	result, err := service.CreateDrill(ctx, CreateCommand{CommandMeta: CommandMeta{RequestID: "feature-create-" + newID("request"), ExpectedVersion: 0}, CreateInput: domain.CreateInput{SiteName: "社区中心", PlannedCapacity: capacity, LeadName: "负责人", ScheduledDate: "2026-08-27"}})
	if err != nil {
		t.Fatal(err)
	}
	return commandDrill(t, result)
}

func freezeFeatureBaseline(t *testing.T, service *Service, ctx context.Context, drill domain.Drill) domain.Drill {
	t.Helper()
	input := BaselinePreviewCommand{Entrances: []string{"东门"}, EvacuationRoutes: []string{"主通道"}, FunctionalZones: []string{"登记区"}, CriticalFacilities: []string{"应急灯"}}
	preview, err := service.PreviewBaseline(ctx, drill.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.FreezeBaseline(ctx, FreezeBaselineCommand{CommandMeta: CommandMeta{RequestID: "feature-freeze-" + newID("request"), ExpectedVersion: drill.Version}, DrillID: drill.ID, Entrances: input.Entrances, EvacuationRoutes: input.EvacuationRoutes, FunctionalZones: input.FunctionalZones, CriticalFacilities: input.CriticalFacilities, PreviewDigest: preview.PreviewDigest})
	if err != nil {
		t.Fatal(err)
	}
	return commandDrill(t, result)
}

func TestDraftRevisionAndBaselinePreview(t *testing.T) {
	service, ctx := featureService(t)
	drill := createFeatureDrill(t, service, ctx, 80)
	invalid := ReviseDrillCommand{CommandMeta: CommandMeta{RequestID: "revision-invalid", ExpectedVersion: drill.Version}, DrillID: drill.ID, CreateInput: domain.CreateInput{SiteName: "", PlannedCapacity: 0, LeadName: "", ScheduledDate: "27-08-2026"}}
	if _, err := service.ReviseDrill(ctx, invalid); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("预期逐字段校验错误，得到 %v", err)
	}
	before, _ := service.GetWorkbench(ctx, drill.ID)
	if before.Aggregate.Drill.Version != drill.Version || before.Aggregate.Drill.PlannedCapacity != 80 {
		t.Fatal("失败修订改变了档案")
	}
	revision := ReviseDrillCommand{CommandMeta: CommandMeta{RequestID: "revision-success", ExpectedVersion: drill.Version}, DrillID: drill.ID, CreateInput: domain.CreateInput{SiteName: "社区中心", PlannedCapacity: 100, LeadName: "新负责人", ScheduledDate: "2026-08-27"}}
	changed, err := service.ReviseDrill(ctx, revision)
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, changed)
	replayed, err := service.ReviseDrill(ctx, revision)
	if err != nil || !replayed.Replayed || string(replayed.Body) != string(changed.Body) {
		t.Fatal("修订幂等重放未返回首次结果")
	}
	if _, err := service.ReviseDrill(ctx, ReviseDrillCommand{CommandMeta: CommandMeta{RequestID: "revision-stale", ExpectedVersion: 1}, DrillID: drill.ID, CreateInput: revision.CreateInput}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("陈旧修订未冲突: %v", err)
	}
	input := BaselinePreviewCommand{Entrances: []string{" 东门 ", "东门", ""}, EvacuationRoutes: []string{"主通道"}, FunctionalZones: []string{"登记区"}, CriticalFacilities: []string{"应急灯"}}
	preview, err := service.PreviewBaseline(ctx, drill.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Baseline.Entrances) != 1 || len(preview.Checkpoints) != 4 || preview.NextVersion != 1 || preview.PreviewDigest == "" {
		t.Fatalf("预览未规范化或缺少确定内容: %+v", preview)
	}
	empty, err := service.PreviewBaseline(ctx, drill.ID, BaselinePreviewCommand{})
	if err != nil || len(empty.ValidationErrors) != 4 {
		t.Fatalf("空预览错误不完整: %+v, %v", empty.ValidationErrors, err)
	}
	changedInput := input
	changedInput.Entrances = []string{"西门"}
	_, err = service.FreezeBaseline(ctx, FreezeBaselineCommand{CommandMeta: CommandMeta{RequestID: "freeze-old-preview", ExpectedVersion: drill.Version}, DrillID: drill.ID, Entrances: changedInput.Entrances, EvacuationRoutes: changedInput.EvacuationRoutes, FunctionalZones: changedInput.FunctionalZones, CriticalFacilities: changedInput.CriticalFacilities, PreviewDigest: preview.PreviewDigest})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("旧摘要冻结未冲突: %v", err)
	}
	current, _ := service.GetWorkbench(ctx, drill.ID)
	if current.Aggregate.Drill.Status != domain.StatusDraft || current.Aggregate.Baseline.Version != 0 {
		t.Fatal("摘要冲突仍写入了基线")
	}
	drill = freezeFeatureBaseline(t, service, ctx, drill)
	if _, err := service.ReviseDrill(ctx, ReviseDrillCommand{CommandMeta: CommandMeta{RequestID: "revision-after-freeze", ExpectedVersion: drill.Version}, DrillID: drill.ID, CreateInput: revision.CreateInput}); !errors.Is(err, domain.ErrInvalidState) {
		t.Fatalf("冻结后仍可修订: %v", err)
	}
}

func TestCheckValidationProgressAndRemediationHistory(t *testing.T) {
	service, ctx := featureService(t)
	drill := freezeFeatureBaseline(t, service, ctx, createFeatureDrill(t, service, ctx, 100))
	base := time.Date(2026, 8, 27, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	input := func(code string, start time.Time, outcome string) domain.ResultInput {
		return domain.ResultInput{CheckpointCode: code, ParticipantCount: 100, StartedAt: start, EndedAt: start.Add(time.Minute), Outcome: outcome, EvidenceDigest: "evidence", RecordedBy: "记录员"}
	}
	over := input("entrance", base, "pass")
	over.ParticipantCount = 101
	if _, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-over-capacity", ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: over}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("超容量未拒绝: %v", err)
	}
	crossDay := input("entrance", base.Add(15*time.Hour), "pass")
	if _, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-cross-day", ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: crossDay}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("跨日未拒绝: %v", err)
	}
	first, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-entrance-valid", ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: input("entrance", base, "pass")})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, first)
	tooEarly := input("zone_guidance", base.Add(30*time.Second), "pass")
	if _, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-bad-order", ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: tooEarly}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("前序时间重叠未拒绝: %v", err)
	}
	checks := []struct {
		code    string
		at      time.Time
		outcome string
	}{{"zone_guidance", base.Add(2 * time.Minute), "pass"}, {"emergency_lighting", base.Add(4 * time.Minute), "fail"}}
	for index, check := range checks {
		result, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-progress-" + check.code, ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: input(check.code, check.at, check.outcome)})
		if err != nil {
			t.Fatalf("检查 %d: %v", index, err)
		}
		drill = commandDrill(t, result)
	}
	view, _ := service.GetWorkbench(ctx, drill.ID)
	if view.Progress.InitialCompleted != 3 || view.Progress.Passed != 2 || view.Progress.Failed != 1 || view.Progress.CompletionPercent != 50 || view.Progress.NextCheckpoint == nil || view.Progress.OpenDeviations != 1 {
		t.Fatalf("进度汇总不符合规则: %+v", view.Progress)
	}
	last, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "check-last", ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: input("supply_access", base.Add(6*time.Minute), "pass")})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, last)
	view, _ = service.GetWorkbench(ctx, drill.ID)
	deviation := view.Aggregate.Deviations[0]
	for materialVersion := 1; materialVersion <= 2; materialVersion++ {
		remediated, err := service.Remediate(ctx, RemediateCommand{CommandMeta: CommandMeta{RequestID: "material-version-" + string(rune('0'+materialVersion)), ExpectedVersion: drill.Version}, DrillID: drill.ID, DeviationID: deviation.ID, RemediationInput: domain.RemediationInput{Cause: "原因", CorrectiveAction: "措施", EvidenceDigest: "material-evidence"}})
		if err != nil {
			t.Fatal(err)
		}
		drill = commandDrill(t, remediated)
		outcome := "fail"
		if materialVersion == 2 {
			outcome = "pass"
		}
		retest := RetestCommand{CommandMeta: CommandMeta{RequestID: "retest-version-" + string(rune('0'+materialVersion)), ExpectedVersion: drill.Version}, DrillID: drill.ID, DeviationID: deviation.ID, ResultInput: input(deviation.CheckpointCode, base.Add(time.Duration(8+materialVersion*2)*time.Minute), outcome)}
		retested, err := service.Retest(ctx, retest)
		if err != nil {
			t.Fatal(err)
		}
		drill = commandDrill(t, retested)
		if materialVersion == 2 {
			replay, err := service.Retest(ctx, retest)
			if err != nil || !replay.Replayed {
				t.Fatal("复验幂等重放失败")
			}
		}
	}
	view, _ = service.GetWorkbench(ctx, drill.ID)
	if len(view.Aggregate.RemediationMaterials) != 2 || len(view.Aggregate.RetestAttempts) != 2 || view.Aggregate.Deviations[0].Status != "closed" || view.Aggregate.Drill.Status != domain.StatusReadyReview {
		t.Fatalf("整改复验历史不完整: %+v", view.Aggregate)
	}
}

func TestReviewReturnResponsesAndResubmission(t *testing.T) {
	service, ctx := featureService(t)
	drill := freezeFeatureBaseline(t, service, ctx, createFeatureDrill(t, service, ctx, 50))
	base := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	for index, code := range []string{"entrance", "zone_guidance", "emergency_lighting", "supply_access"} {
		result, err := service.RecordCheck(ctx, RecordCheckCommand{CommandMeta: CommandMeta{RequestID: "review-check-" + code, ExpectedVersion: drill.Version}, DrillID: drill.ID, ResultInput: domain.ResultInput{CheckpointCode: code, ParticipantCount: 50, StartedAt: base.Add(time.Duration(index) * 2 * time.Minute), EndedAt: base.Add(time.Duration(index)*2*time.Minute + time.Minute), Outcome: "pass", EvidenceDigest: "evidence", RecordedBy: "记录员"}})
		if err != nil {
			t.Fatal(err)
		}
		drill = commandDrill(t, result)
	}
	submitted, err := service.SubmitReview(ctx, SubmitReviewCommand{CommandMeta: CommandMeta{RequestID: "review-submit-first", ExpectedVersion: drill.Version}, DrillID: drill.ID})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, submitted)
	withoutItems := ReviewCommand{CommandMeta: CommandMeta{RequestID: "review-empty-return", ExpectedVersion: drill.Version}, DrillID: drill.ID, Decision: "rejected", ReviewerName: "复核员", ReviewNote: "需处理"}
	if _, err := service.Review(ctx, withoutItems); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("无事项退回未拒绝: %v", err)
	}
	items := []domain.ReviewItemInput{{Description: "补充入口标识", ReferenceType: "checkpoint", ReferenceValue: "entrance", Requirement: "提供现场照片"}, {Description: "确认物资编号", ReferenceType: "baseline_item", ReferenceValue: "应急灯", Requirement: "提供清册"}}
	returned, err := service.Review(ctx, ReviewCommand{CommandMeta: CommandMeta{RequestID: "review-return-items", ExpectedVersion: drill.Version}, DrillID: drill.ID, Decision: "rejected", ReviewerName: "复核员", ReviewNote: "按事项整改", Items: items})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, returned)
	view, _ := service.GetWorkbench(ctx, drill.ID)
	round := view.Aggregate.ReviewRounds[0]
	responded, err := service.RespondToReview(ctx, ReviewResponsesCommand{CommandMeta: CommandMeta{RequestID: "review-response-one", ExpectedVersion: drill.Version}, DrillID: drill.ID, Responses: []domain.ReviewResponseInput{{ItemID: round.Items[0].ID, Response: "已补充", EvidenceDigest: "photo-digest"}}})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, responded)
	if _, err := service.SubmitReview(ctx, SubmitReviewCommand{CommandMeta: CommandMeta{RequestID: "review-submit-incomplete", ExpectedVersion: drill.Version}, DrillID: drill.ID}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("缺项再次送审未拒绝: %v", err)
	}
	responded, err = service.RespondToReview(ctx, ReviewResponsesCommand{CommandMeta: CommandMeta{RequestID: "review-response-two", ExpectedVersion: drill.Version}, DrillID: drill.ID, Responses: []domain.ReviewResponseInput{{ItemID: round.Items[1].ID, Response: "已核对", EvidenceDigest: "inventory-digest"}}})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, responded)
	resubmitted, err := service.SubmitReview(ctx, SubmitReviewCommand{CommandMeta: CommandMeta{RequestID: "review-submit-second", ExpectedVersion: drill.Version}, DrillID: drill.ID})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, resubmitted)
	approved, err := service.Review(ctx, ReviewCommand{CommandMeta: CommandMeta{RequestID: "review-approve", ExpectedVersion: drill.Version}, DrillID: drill.ID, Decision: "approved", ReviewerName: "复核员", ReviewNote: "回应完整"})
	if err != nil {
		t.Fatal(err)
	}
	drill = commandDrill(t, approved)
	view, _ = service.GetWorkbench(ctx, drill.ID)
	if drill.Status != domain.StatusActivated || len(view.Aggregate.ReviewRounds) != 1 || !view.Aggregate.ReviewRounds[0].ResponsesFrozen || view.Aggregate.ReviewRounds[0].ResubmittedAt == nil {
		t.Fatalf("复核轮次未冻结: %+v", view.Aggregate.ReviewRounds)
	}
	verification, err := service.VerifyDecision(ctx, drill.ID)
	if err != nil || !verification.Valid || verification.DocumentDigest == "" {
		t.Fatalf("决定书校验失败: %+v %v", verification, err)
	}
}
