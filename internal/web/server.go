package web

import (
	"embed"
	"net/http"
	"strings"

	"shelter-drill-gate/internal/application"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	service *application.Service
}

func NewServer(service *application.Service) *Server { return &Server{service: service} }

func (s *Server) Handler() http.Handler { return securityHeaders(http.HandlerFunc(s.ServeHTTP)) }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" && r.Method == http.MethodGet {
		s.WorkbenchHandler(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/assets/") && r.Method == http.MethodGet {
		s.AssetHandler(w, r)
		return
	}
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		s.HealthHandler(w, r)
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "route_not_found", "请求路径不存在", nil)
		return
	}
	segments := splitPath(r.URL.Path)
	if len(segments) == 2 && segments[1] == "drills" {
		switch r.Method {
		case http.MethodGet:
			s.ListDrillsHandler(w, r)
		case http.MethodPost:
			s.CreateDrillHandler(w, r)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(segments) < 3 || segments[1] != "drills" {
		writeError(w, http.StatusNotFound, "route_not_found", "请求路径不存在", nil)
		return
	}
	id := segments[2]
	if len(segments) == 3 {
		switch r.Method {
		case http.MethodGet:
			s.DrillDetailHandler(w, r, id)
		case http.MethodPatch:
			s.ReviseDrillHandler(w, r, id)
		default:
			methodNotAllowed(w)
		}
		return
	}
	if len(segments) == 5 && segments[3] == "baseline" && segments[4] == "preview" && r.Method == http.MethodPost {
		s.PreviewBaselineHandler(w, r, id)
		return
	}
	if len(segments) == 5 && segments[3] == "baseline" && segments[4] == "freeze" && r.Method == http.MethodPost {
		s.FreezeBaselineHandler(w, r, id)
		return
	}
	if len(segments) == 6 && segments[3] == "checkpoints" && segments[5] == "results" && r.Method == http.MethodPost {
		s.RecordCheckpointHandler(w, r, id, segments[4])
		return
	}
	if len(segments) == 6 && segments[3] == "deviations" && segments[5] == "remediation" && r.Method == http.MethodPost {
		s.RemediateHandler(w, r, id, segments[4])
		return
	}
	if len(segments) == 6 && segments[3] == "deviations" && segments[5] == "retest" && r.Method == http.MethodPost {
		s.RetestHandler(w, r, id, segments[4])
		return
	}
	if len(segments) == 5 && segments[3] == "review" && segments[4] == "submit" && r.Method == http.MethodPost {
		s.SubmitReviewHandler(w, r, id)
		return
	}
	if len(segments) == 5 && segments[3] == "review" && segments[4] == "decision" && r.Method == http.MethodPost {
		s.ReviewDecisionHandler(w, r, id)
		return
	}
	if len(segments) == 5 && segments[3] == "review" && segments[4] == "responses" && r.Method == http.MethodPost {
		s.ReviewResponsesHandler(w, r, id)
		return
	}
	if len(segments) == 4 && segments[3] == "timeline" && r.Method == http.MethodGet {
		s.TimelineHandler(w, r, id)
		return
	}
	if len(segments) == 5 && segments[3] == "decision" && segments[4] == "verify" && r.Method == http.MethodGet {
		s.VerifyDecisionHandler(w, r, id)
		return
	}
	if len(segments) == 5 && segments[3] == "decision" && segments[4] == "export" && r.Method == http.MethodGet {
		s.ExportDecisionHandler(w, r, id)
		return
	}
	writeError(w, http.StatusNotFound, "route_not_found", "请求路径不存在", nil)
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self' data:; connect-src 'self'")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
