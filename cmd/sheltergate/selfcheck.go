package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type drillEnvelope struct {
	Drill struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
		Status  string `json:"status"`
	} `json:"drill"`
}
type workbenchEnvelope struct {
	Aggregate struct {
		Drill struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
			Status  string `json:"status"`
		} `json:"drill"`
		Decision *struct {
			DocumentDigest string `json:"document_digest"`
		} `json:"decision"`
	} `json:"aggregate"`
	TimelineValid bool  `json:"timeline_valid"`
	DecisionValid bool  `json:"decision_valid"`
	Timeline      []any `json:"timeline"`
}

func runSelfCheck(cfg config) error {
	tempDir, err := os.MkdirTemp("", "sheltergate-selfcheck-")
	if err != nil {
		return fmt.Errorf("创建自检目录: %w", err)
	}
	defer os.RemoveAll(tempDir)
	cfg.Database = filepath.Join(tempDir, "selfcheck.db")
	runtime, err := buildRuntime(cfg)
	if err != nil {
		return err
	}
	done := runtime.serve()
	client := newSmokeClient(runtime.listener.Addr().String())
	flowErr := executeSmokeFlow(client)
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	closeErr := runtime.close(ctx)
	cancel()
	serveErr := <-done
	if flowErr != nil {
		return flowErr
	}
	if closeErr != nil {
		return closeErr
	}
	if serveErr != nil {
		return serveErr
	}
	fmt.Println("selfcheck: 草稿修订、基线预览、完整演练、独立复核和稳定决定书导出通过")
	return nil
}

func executeSmokeFlow(client *smokeClient) error {
	var created drillEnvelope
	if err := client.command(http.MethodPost, "/api/drills", 0, map[string]any{"site_name": "自检社区避难中心", "planned_capacity": 320, "lead_name": "演练负责人", "scheduled_date": "2026-08-27"}, &created); err != nil {
		return err
	}
	id, version := created.Drill.ID, created.Drill.Version
	var changed drillEnvelope
	if err := client.command(http.MethodPatch, "/api/drills/"+id, version, map[string]any{"site_name": "自检社区避难中心", "planned_capacity": 321, "lead_name": "演练负责人", "scheduled_date": "2026-08-27"}, &changed); err != nil {
		return err
	}
	version = changed.Drill.Version
	baseline := map[string]any{"entrances": []string{"东侧入口", "西侧入口", "东侧入口"}, "evacuation_routes": []string{"东门至登记区", "西门至物资区"}, "functional_zones": []string{"登记区", "安置区"}, "critical_facilities": []string{"应急照明", "急救物资"}}
	var preview struct {
		PreviewDigest string `json:"preview_digest"`
		Checkpoints   []any  `json:"checkpoints"`
	}
	if err := client.request(http.MethodPost, "/api/drills/"+id+"/baseline/preview", baseline, &preview); err != nil {
		return err
	}
	if preview.PreviewDigest == "" || len(preview.Checkpoints) != 4 {
		return fmt.Errorf("基线预览缺少摘要或检查点")
	}
	baseline["preview_digest"] = preview.PreviewDigest
	if err := client.command(http.MethodPost, "/api/drills/"+id+"/baseline/freeze", version, baseline, &changed); err != nil {
		return err
	}
	version = changed.Drill.Version
	checkpoints := []string{"entrance", "zone_guidance", "emergency_lighting", "supply_access"}
	base := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	for index, code := range checkpoints {
		started := base.Add(time.Duration(index) * 10 * time.Minute)
		ended := started.Add(45 * time.Second)
		if err := client.command(http.MethodPost, "/api/drills/"+id+"/checkpoints/"+code+"/results", version, map[string]any{"participant_count": 80, "started_at": started.Format(time.RFC3339), "ended_at": ended.Format(time.RFC3339), "outcome": "pass", "evidence_digest": "selfcheck-evidence-" + code, "recorded_by": "现场记录员"}, &changed); err != nil {
			return err
		}
		version = changed.Drill.Version
	}
	if changed.Drill.Status != "ready_for_review" {
		return fmt.Errorf("检查完成后的状态为 %s", changed.Drill.Status)
	}
	if err := client.command(http.MethodPost, "/api/drills/"+id+"/review/submit", version, map[string]any{}, &changed); err != nil {
		return err
	}
	version = changed.Drill.Version
	if err := client.command(http.MethodPost, "/api/drills/"+id+"/review/decision", version, map[string]any{"decision": "approved", "reviewer_name": "独立安全复核员", "review_note": "现场布设与检查记录符合启用要求"}, &changed); err != nil {
		return err
	}
	var workbench workbenchEnvelope
	if err := client.get("/api/drills/"+id, &workbench); err != nil {
		return err
	}
	if workbench.Aggregate.Drill.Status != "activated" || workbench.Aggregate.Decision == nil {
		return fmt.Errorf("未生成启用决定")
	}
	if !workbench.TimelineValid || !workbench.DecisionValid {
		return fmt.Errorf("摘要校验失败: timeline=%t decision=%t", workbench.TimelineValid, workbench.DecisionValid)
	}
	var verification struct {
		Valid          bool   `json:"valid"`
		DocumentDigest string `json:"document_digest"`
	}
	if err := client.get("/api/drills/"+id+"/decision/verify", &verification); err != nil {
		return err
	}
	if !verification.Valid || verification.DocumentDigest == "" {
		return fmt.Errorf("决定书校验端点未返回有效结果")
	}
	firstExport, err := client.getBytes("/api/drills/" + id + "/decision/export")
	if err != nil {
		return err
	}
	secondExport, err := client.getBytes("/api/drills/" + id + "/decision/export")
	if err != nil {
		return err
	}
	if !bytes.Equal(firstExport, secondExport) {
		return fmt.Errorf("重复导出的决定书内容不一致")
	}
	var afterExport workbenchEnvelope
	if err := client.get("/api/drills/"+id, &afterExport); err != nil {
		return err
	}
	if afterExport.Aggregate.Drill.Version != workbench.Aggregate.Drill.Version || len(afterExport.Timeline) != len(workbench.Timeline) {
		return fmt.Errorf("只读导出改变了演练版本或审计时间线")
	}
	return nil
}
