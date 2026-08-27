package web

import (
	"encoding/json"
	"net/http"

	"shelter-drill-gate/internal/application"
	"shelter-drill-gate/internal/domain"
)

func (s *Server) ListDrillsHandler(w http.ResponseWriter, r *http.Request) {
	drills, err := s.service.ListDrills(r.Context())
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	if drills == nil {
		drills = []domain.Drill{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"drills": drills})
}

func (s *Server) CreateDrillHandler(w http.ResponseWriter, r *http.Request) {
	var command application.CreateCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	result, err := s.service.CreateDrill(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) ReviseDrillHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.ReviseDrillCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	result, err := s.service.ReviseDrill(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) DrillDetailHandler(w http.ResponseWriter, r *http.Request, id string) {
	view, err := s.service.GetWorkbench(r.Context(), id)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) FreezeBaselineHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.FreezeBaselineCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	result, err := s.service.FreezeBaseline(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) PreviewBaselineHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.BaselinePreviewCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	preview, err := s.service.PreviewBaseline(r.Context(), id, command)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) RecordCheckpointHandler(w http.ResponseWriter, r *http.Request, id, code string) {
	var command application.RecordCheckCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	command.CheckpointCode = code
	result, err := s.service.RecordCheck(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) RemediateHandler(w http.ResponseWriter, r *http.Request, id, deviationID string) {
	var command application.RemediateCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	command.DeviationID = deviationID
	result, err := s.service.Remediate(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) RetestHandler(w http.ResponseWriter, r *http.Request, id, deviationID string) {
	var command application.RetestCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	command.DeviationID = deviationID
	result, err := s.service.Retest(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) SubmitReviewHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.SubmitReviewCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	result, err := s.service.SubmitReview(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) ReviewDecisionHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.ReviewCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	result, err := s.service.Review(r.Context(), command)
	writeCommandResult(w, result, err)
}

func (s *Server) ReviewResponsesHandler(w http.ResponseWriter, r *http.Request, id string) {
	var command application.ReviewResponsesCommand
	if !decodeJSON(w, r, &command) {
		return
	}
	command.DrillID = id
	result, err := s.service.RespondToReview(r.Context(), command)
	writeCommandResult(w, result, err)
}

func writeCommandResult(w http.ResponseWriter, result application.CommandResult, err error) {
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if result.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Body)
}

func (s *Server) TimelineHandler(w http.ResponseWriter, r *http.Request, id string) {
	view, err := s.service.GetWorkbench(r.Context(), id)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"timeline": view.Timeline, "timeline_valid": view.TimelineValid})
}

func (s *Server) VerifyDecisionHandler(w http.ResponseWriter, r *http.Request, id string) {
	verification, err := s.service.VerifyDecision(r.Context(), id)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, verification)
}

type decisionExport struct {
	Document       domain.DecisionDocument `json:"document"`
	DocumentDigest string                  `json:"document_digest"`
}

func (s *Server) ExportDecisionHandler(w http.ResponseWriter, r *http.Request, id string) {
	verification, err := s.service.VerifyDecision(r.Context(), id)
	if err != nil {
		writeApplicationError(w, err)
		return
	}
	if !verification.Valid {
		writeError(w, http.StatusConflict, "decision_invalid", "决定书未通过归档校验", verification.Errors)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="activation-decision-`+id+`.json"`)
	_ = json.NewEncoder(w).Encode(decisionExport{Document: verification.Document, DocumentDigest: verification.DocumentDigest})
}
