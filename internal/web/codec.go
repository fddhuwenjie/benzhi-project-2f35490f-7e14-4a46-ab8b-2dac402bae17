package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"shelter-drill-gate/internal/domain"
)

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Header.Get("Content-Type") != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "content_type", "Content-Type 必须为 application/json", nil)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "JSON 请求内容无效", nil)
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求只能包含一个 JSON 对象", nil)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string, details any) {
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "details": details}})
}

func writeApplicationError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationErrors
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", "提交内容未通过校验", validation.Items)
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "version_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrIdempotencyKey):
		writeError(w, http.StatusConflict, "request_id_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", err.Error(), nil)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrDecisionUnavailable):
		writeError(w, http.StatusConflict, "decision_unavailable", err.Error(), nil)
	case errors.Is(err, domain.ErrValidation):
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "服务暂时无法处理请求", nil)
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不受支持", nil)
}

func writeCommand(w http.ResponseWriter, result interface{ GetStatus() int }) { _ = result }
