package web

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMountCRM_ServesIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := MountCRM(r); err != nil {
		t.Fatalf("MountCRM: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/crm/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /crm/ status = %d, want 200 (body len %d)", w.Code, w.Body.Len())
	}
	body := w.Body.String()
	if !strings.Contains(body, "Job Hunt CRM") {
		t.Fatalf("expected CRM HTML, got: %s", truncTest(body, 200))
	}
}

func TestMountCRM_ServesNextAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root, fromDisk, err := resolveCRMRoot()
	if err != nil {
		t.Skip(err)
	}
	var sub fs.FS
	if fromDisk {
		sub = os.DirFS(root)
	} else {
		sub, _ = fs.Sub(crmAssets, "static/crm")
	}
	var assetPath string
	_ = fs.WalkDir(sub, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || assetPath != "" {
			return err
		}
		if strings.HasPrefix(path, "_next/") && strings.HasSuffix(path, ".js") {
			assetPath = path
		}
		return nil
	})
	if assetPath == "" {
		t.Skip("no _next js asset (run make build-crm)")
	}

	r := gin.New()
	if err := MountCRM(r); err != nil {
		t.Fatalf("MountCRM: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/crm/"+assetPath, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /crm/%s status = %d", assetPath, w.Code)
	}
}

func truncTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
