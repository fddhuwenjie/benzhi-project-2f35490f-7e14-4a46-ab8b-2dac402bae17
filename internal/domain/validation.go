package domain

import (
	"strings"
	"time"
)

type CreateInput struct {
	SiteName        string `json:"site_name"`
	PlannedCapacity int    `json:"planned_capacity"`
	LeadName        string `json:"lead_name"`
	ScheduledDate   string `json:"scheduled_date"`
}

func ValidateCreate(in CreateInput) error {
	var problems ValidationErrors
	if strings.TrimSpace(in.SiteName) == "" {
		problems.Add("site_name", "请填写场所名称")
	}
	if in.PlannedCapacity <= 0 || in.PlannedCapacity > 100000 {
		problems.Add("planned_capacity", "容量必须在 1 到 100000 之间")
	}
	if strings.TrimSpace(in.LeadName) == "" {
		problems.Add("lead_name", "请填写演练负责人")
	}
	if _, err := time.Parse("2006-01-02", in.ScheduledDate); err != nil {
		problems.Add("scheduled_date", "演练日期格式应为 YYYY-MM-DD")
	}
	return problems.Err()
}

func normalizeList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" && !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func NormalizeCreate(in CreateInput) CreateInput {
	in.SiteName = strings.TrimSpace(in.SiteName)
	in.LeadName = strings.TrimSpace(in.LeadName)
	in.ScheduledDate = strings.TrimSpace(in.ScheduledDate)
	return in
}

func NormalizeBaseline(b LayoutBaseline) LayoutBaseline {
	b.Entrances = normalizeList(b.Entrances)
	b.EvacuationRoutes = normalizeList(b.EvacuationRoutes)
	b.FunctionalZones = normalizeList(b.FunctionalZones)
	b.CriticalFacilities = normalizeList(b.CriticalFacilities)
	return b
}

func ValidateBaseline(b LayoutBaseline) error {
	var problems ValidationErrors
	if len(b.Entrances) == 0 {
		problems.Add("entrances", "至少登记一个疏散入口")
	}
	if len(b.EvacuationRoutes) == 0 {
		problems.Add("evacuation_routes", "至少登记一条疏散路径")
	}
	if len(b.FunctionalZones) == 0 {
		problems.Add("functional_zones", "至少登记一个功能分区")
	}
	if len(b.CriticalFacilities) == 0 {
		problems.Add("critical_facilities", "至少登记一项关键设施")
	}
	return problems.Err()
}
