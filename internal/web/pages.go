package web

import (
	"io/fs"
	"net/http"
	"path"
)

func (s *Server) WorkbenchHandler(w http.ResponseWriter, r *http.Request) {
	raw, err := staticFiles.ReadFile("static/index.html")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "asset_error", "工作台资源不可用", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *Server) AssetHandler(w http.ResponseWriter, r *http.Request) {
	name := path.Base(r.URL.Path)
	if name != "app.css" && name != "features.css" && name != "app.js" {
		http.NotFound(w, r)
		return
	}
	raw, err := fs.ReadFile(staticFiles, "static/"+name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if path.Ext(name) == ".css" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(raw)
}

func (s *Server) HealthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
