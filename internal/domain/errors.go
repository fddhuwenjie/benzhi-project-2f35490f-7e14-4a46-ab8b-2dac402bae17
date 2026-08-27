package domain

import "errors"

var (
	ErrNotFound            = errors.New("未找到演练档案")
	ErrConflict            = errors.New("数据版本已变化，请刷新后重试")
	ErrInvalidState        = errors.New("当前状态不允许执行此操作")
	ErrValidation          = errors.New("提交内容不完整或不符合规则")
	ErrIdempotencyKey      = errors.New("request_id 已用于其他操作")
	ErrDecisionUnavailable = errors.New("当前演练没有可导出的启用决定书")
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors struct {
	Items []FieldError `json:"items"`
}

func (e *ValidationErrors) Error() string { return ErrValidation.Error() }

func (e *ValidationErrors) Unwrap() error { return ErrValidation }

func (e *ValidationErrors) Add(field, message string) {
	e.Items = append(e.Items, FieldError{Field: field, Message: message})
}

func (e *ValidationErrors) Err() error {
	if len(e.Items) == 0 {
		return nil
	}
	return e
}
