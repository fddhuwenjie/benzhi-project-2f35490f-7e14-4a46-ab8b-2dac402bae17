package audit_head_cross_store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"shelter-drill-gate/internal/audit"
	"shelter-drill-gate/internal/domain"
	"shelter-drill-gate/internal/storage"
)

func TestAuditHeadCacheIsScopedToStore(t *testing.T) {
	first := openStore(t, "first.db")
	second := openStore(t, "second.db")
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	const sharedID = "drill-same-public-id"

	insertDrill(t, first, sharedID, now)
	appendEvent(t, first, sharedID, "first-store-created", now)
	insertDrill(t, second, sharedID, now)
	appendEvent(t, second, sharedID, "second-store-created", now)

	events, err := second.LoadEvents(context.Background(), sharedID)
	if err != nil {
		t.Fatal(err)
	}
	if err := audit.Verify(events); err != nil {
		t.Fatalf("第二个 Store 的审计时间线受其他数据库污染: %v", err)
	}
	if len(events) != 1 || events[0].Sequence != 1 || events[0].PreviousHash != "" {
		t.Fatalf("第二个 Store 的首事件头错误: %+v", events)
	}
}

func openStore(t *testing.T, name string) *storage.Store {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func insertDrill(t *testing.T, store *storage.Store, id string, now time.Time) {
	t.Helper()
	err := store.WithinTx(context.Background(), func(tx *storage.Tx) error {
		return tx.InsertDrill(context.Background(), domain.Drill{
			ID: id, SiteName: id, PlannedCapacity: 10, LeadName: "负责人",
			ScheduledDate: "2026-08-27", Status: domain.StatusDraft, Version: 1,
			CreatedAt: now, UpdatedAt: now,
		})
	})
	if err != nil {
		t.Fatal(err)
	}
}

func appendEvent(t *testing.T, store *storage.Store, drillID, eventType string, now time.Time) {
	t.Helper()
	err := store.WithinTx(context.Background(), func(tx *storage.Tx) error {
		_, err := tx.AppendEvent(context.Background(), drillID, eventType, map[string]any{"event_type": eventType}, now)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
