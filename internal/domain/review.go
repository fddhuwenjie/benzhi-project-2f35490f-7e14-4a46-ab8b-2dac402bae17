package domain

import (
	"fmt"
	"strings"
	"time"
)

type ReviewItemInput struct {
	Description    string `json:"description"`
	ReferenceType  string `json:"reference_type"`
	ReferenceValue string `json:"reference_value"`
	Requirement    string `json:"requirement"`
}

type ReviewResponseInput struct {
	ItemID         string `json:"item_id"`
	Response       string `json:"response"`
	EvidenceDigest string `json:"evidence_digest"`
}

func BuildReviewRound(round int, reviewer, note string, inputs []ReviewItemInput, now time.Time, id func() string) (ReviewRound, error) {
	var problems ValidationErrors
	if len(inputs) == 0 {
		problems.Add("items", "退回时必须至少提交一条结构化事项")
	}
	items := make([]ReviewItem, 0, len(inputs))
	for index, input := range inputs {
		description := strings.TrimSpace(input.Description)
		referenceType := strings.TrimSpace(input.ReferenceType)
		referenceValue := strings.TrimSpace(input.ReferenceValue)
		requirement := strings.TrimSpace(input.Requirement)
		if description == "" {
			problems.Add(fmt.Sprintf("items[%d].description", index), "请填写问题描述")
		}
		if requirement == "" {
			problems.Add(fmt.Sprintf("items[%d].requirement", index), "请填写处理要求")
		}
		if referenceType != "checkpoint" && referenceType != "baseline_item" {
			problems.Add(fmt.Sprintf("items[%d].reference_type", index), "关联类型必须为 checkpoint 或 baseline_item")
		}
		if referenceValue == "" {
			problems.Add(fmt.Sprintf("items[%d].reference_value", index), "请填写关联检查点或基线项目")
		}
		items = append(items, ReviewItem{ID: id(), Round: round, Description: description, ReferenceType: referenceType, ReferenceValue: referenceValue, Requirement: requirement})
	}
	if err := problems.Err(); err != nil {
		return ReviewRound{}, err
	}
	return ReviewRound{Round: round, ReviewerName: strings.TrimSpace(reviewer), ReviewNote: strings.TrimSpace(note), ReturnedAt: now.UTC(), Items: items}, nil
}

func CurrentOpenReviewRound(a *Aggregate) *ReviewRound {
	for index := len(a.ReviewRounds) - 1; index >= 0; index-- {
		if !a.ReviewRounds[index].ResponsesFrozen {
			return &a.ReviewRounds[index]
		}
	}
	return nil
}

func ApplyReviewResponses(round *ReviewRound, inputs []ReviewResponseInput, now time.Time) error {
	if round == nil || round.ResponsesFrozen {
		return ErrInvalidState
	}
	var problems ValidationErrors
	seen := make(map[string]bool)
	for index, input := range inputs {
		if seen[input.ItemID] {
			problems.Add(fmt.Sprintf("responses[%d].item_id", index), "同一事项不能重复回应")
			continue
		}
		seen[input.ItemID] = true
		var item *ReviewItem
		for i := range round.Items {
			if round.Items[i].ID == input.ItemID {
				item = &round.Items[i]
				break
			}
		}
		if item == nil {
			problems.Add(fmt.Sprintf("responses[%d].item_id", index), "复核事项不存在于当前轮次")
			continue
		}
		response := strings.TrimSpace(input.Response)
		evidence := strings.TrimSpace(input.EvidenceDigest)
		if response == "" {
			problems.Add(fmt.Sprintf("responses[%d].response", index), "请填写事项回应")
		}
		if evidence == "" {
			problems.Add(fmt.Sprintf("responses[%d].evidence_digest", index), "请填写回应证据摘要")
		}
		if response != "" && evidence != "" {
			item.Response = ReviewResponse{Response: response, EvidenceDigest: evidence, RespondedAt: now.UTC()}
		}
	}
	return problems.Err()
}

func FreezeReviewResponses(round *ReviewRound, now time.Time) error {
	if round == nil || round.ResponsesFrozen {
		return ErrInvalidState
	}
	var problems ValidationErrors
	for _, item := range round.Items {
		if strings.TrimSpace(item.Response.Response) == "" {
			problems.Add("items."+item.ID+".response", "复核事项尚未回应")
		}
		if strings.TrimSpace(item.Response.EvidenceDigest) == "" {
			problems.Add("items."+item.ID+".evidence_digest", "复核事项缺少回应证据")
		}
	}
	if err := problems.Err(); err != nil {
		return err
	}
	frozenAt := now.UTC()
	round.ResponsesFrozen = true
	round.ResubmittedAt = &frozenAt
	return nil
}
