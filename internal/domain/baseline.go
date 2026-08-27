package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type BaselinePreview struct {
	Baseline         LayoutBaseline `json:"baseline"`
	Checkpoints      []Checkpoint   `json:"checkpoints"`
	NextVersion      int            `json:"next_version"`
	ContentDigest    string         `json:"content_digest"`
	PreviewDigest    string         `json:"preview_digest"`
	DrillVersion     int            `json:"drill_version"`
	ValidationErrors []FieldError   `json:"validation_errors"`
}

func PreviewBaseline(a *Aggregate, proposed LayoutBaseline) BaselinePreview {
	b := NormalizeBaseline(proposed)
	b.DrillID = a.Drill.ID
	b.Version = a.Drill.BaselineVersion + 1
	contentDigest := baselineDigest(b)
	b.ContentDigest = contentDigest
	previewDigest := digestValue(struct {
		DrillID      string         `json:"drill_id"`
		DrillVersion int            `json:"drill_version"`
		Baseline     LayoutBaseline `json:"baseline"`
	}{a.Drill.ID, a.Drill.Version, b})
	preview := BaselinePreview{Baseline: b, Checkpoints: GenerateCheckpoints(b), NextVersion: b.Version, ContentDigest: contentDigest, PreviewDigest: previewDigest, DrillVersion: a.Drill.Version}
	if err := ValidateBaseline(b); err != nil {
		if validation, ok := err.(*ValidationErrors); ok {
			preview.ValidationErrors = validation.Items
		}
	}
	return preview
}

func FreezeBaseline(a *Aggregate, now time.Time) error {
	if a.Drill.Status != StatusDraft {
		return ErrInvalidState
	}
	b := NormalizeBaseline(a.Baseline)
	if err := ValidateBaseline(b); err != nil {
		return err
	}
	b.Version = a.Drill.BaselineVersion + 1
	b.DrillID = a.Drill.ID
	b.FrozenAt = now.UTC()
	b.ContentDigest = baselineDigest(b)
	a.Baseline = b
	a.Drill.BaselineVersion = b.Version
	a.Checkpoints = GenerateCheckpoints(b)
	a.Drill.Status = StatusBaselineFrozen
	return nil
}

func baselineDigest(b LayoutBaseline) string {
	canonical := struct {
		Version            int      `json:"version"`
		Entrances          []string `json:"entrances"`
		EvacuationRoutes   []string `json:"evacuation_routes"`
		FunctionalZones    []string `json:"functional_zones"`
		CriticalFacilities []string `json:"critical_facilities"`
	}{b.Version, b.Entrances, b.EvacuationRoutes, b.FunctionalZones, b.CriticalFacilities}
	raw, _ := json.Marshal(canonical)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func digestValue(value any) string {
	raw, _ := json.Marshal(value)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func GenerateCheckpoints(b LayoutBaseline) []Checkpoint {
	return []Checkpoint{
		{Code: "entrance", Order: 1, Name: "疏散入口", Requirement: fmt.Sprintf("核验 %d 个入口畅通且人员通过有序", len(b.Entrances)), MaxSeconds: 180},
		{Code: "zone_guidance", Order: 2, Name: "分区引导", Requirement: fmt.Sprintf("核验 %d 个功能分区的引导标识和路径", len(b.FunctionalZones)), MaxSeconds: 240},
		{Code: "emergency_lighting", Order: 3, Name: "应急照明", Requirement: "核验疏散路径与关键设施照明可用", MaxSeconds: 120},
		{Code: "supply_access", Order: 4, Name: "物资可达性", Requirement: fmt.Sprintf("核验 %d 类关键设施及物资能够快速取得", len(b.CriticalFacilities)), MaxSeconds: 180},
	}
}
