package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"shelter-drill-gate/internal/application"
	"shelter-drill-gate/internal/storage"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return httptest.NewServer(NewServer(application.NewService(store)).Handler())
}

func TestWorkbenchAndSecurityHeaders(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	response, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("状态码 %d", response.StatusCode)
	}
	if response.Header.Get("Content-Security-Policy") == "" || response.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatal("安全响应头缺失")
	}
	if response.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("内容类型错误: %s", response.Header.Get("Content-Type"))
	}
}

func TestCreateRejectsUnknownJSONField(t *testing.T) {
	server := testServer(t)
	defer server.Close()
	body := `{"request_id":"request-web","expected_version":0,"site_name":"站点","planned_capacity":10,"lead_name":"负责人","scheduled_date":"2026-08-27","unknown":true}`
	response, err := http.Post(server.URL+"/api/drills", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("状态码 %d", response.StatusCode)
	}
}
