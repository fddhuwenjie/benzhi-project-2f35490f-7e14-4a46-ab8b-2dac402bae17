package aggregate_load_error_chain_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"modernc.org/sqlite"

	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

func TestAggregateLoadPreservesStageErrorChain(t *testing.T) {
	stages := []struct {
		name            string
		table           string
		baselineVersion int
	}{
		{name: "baseline", table: "baselines", baselineVersion: 1},
		{name: "checkpoints", table: "checkpoints"},
		{name: "results", table: "checkpoint_results"},
		{name: "deviations", table: "deviations"},
		{name: "remediation", table: "remediation_materials"},
		{name: "retests", table: "retest_attempts"},
		{name: "reviews", table: "review_rounds"},
		{name: "decision", table: "activation_decisions"},
	}

	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stage.db")
			store, err := storage.Open(path)
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			t.Cleanup(func() { _ = store.Close() })

			now := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
			drill := domain.Drill{
				ID: "drill-error-chain", SiteName: "错误链测试场所", PlannedCapacity: 80,
				LeadName: "负责人", ScheduledDate: "2026-08-27", Status: domain.StatusDraft,
				BaselineVersion: stage.baselineVersion, Version: 1, CreatedAt: now, UpdatedAt: now,
			}
			if err := store.WithinTx(context.Background(), func(tx *storage.Tx) error {
				return tx.InsertDrill(context.Background(), drill)
			}); err != nil {
				t.Fatalf("seed drill: %v", err)
			}

			raw, err := sql.Open("sqlite", "file:"+path)
			if err != nil {
				t.Fatalf("open raw database: %v", err)
			}
			if _, err := raw.ExecContext(context.Background(), "DROP TABLE "+stage.table); err != nil {
				_ = raw.Close()
				t.Fatalf("drop %s: %v", stage.table, err)
			}
			if err := raw.Close(); err != nil {
				t.Fatalf("close raw database: %v", err)
			}

			_, err = store.LoadAggregate(context.Background(), drill.ID)
			if err == nil {
				t.Fatalf("load unexpectedly succeeded after dropping %s", stage.table)
			}
			var sqliteErr *sqlite.Error
			if !errors.As(err, &sqliteErr) {
				t.Fatalf("%s stage discarded sqlite error chain: %v", stage.name, err)
			}
		})
	}
}
