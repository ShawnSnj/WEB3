package web

import (
	"embed"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// all: includes build artifacts that may be gitignored (see Makefile build-crm).
//
//go:embed all:static/crm
var crmAssets embed.FS

const crmBasePath = "/crm"

// MountCRM serves the Next.js static export at /crm/.
func MountCRM(r *gin.Engine) error {
	root, fromDisk, err := resolveCRMRoot()
	if err != nil {
		return err
	}
	var sub fs.FS
	if fromDisk {
		sub = os.DirFS(root)
	} else {
		sub, err = fs.Sub(crmAssets, "static/crm")
		if err != nil {
			return fmt.Errorf("crm static fs: %w", err)
		}
	}

	redirectToCRM := func(c *gin.Context) {
		c.Redirect(http.StatusFound, crmBasePath+"/")
	}

	r.GET("/crm", redirectToCRM)
	r.HEAD("/crm", redirectToCRM)

	serve := func(c *gin.Context) {
		rel, ok := crmRelativePath(c.Param("filepath"))
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		if st, err := fs.Stat(sub, rel); err != nil {
			// Unknown client route — SPA shell.
			rel = "index.html"
		} else if st.IsDir() {
			rel = strings.TrimSuffix(rel, "/") + "/index.html"
		}
		serveCRMFile(c, sub, rel)
	}

	r.GET("/crm/*filepath", serve)
	r.HEAD("/crm/*filepath", serve)

	return nil
}

func resolveCRMRoot() (string, bool, error) {
	// Prefer on-disk build (make build-crm) so go run picks up fresh assets.
	candidates := []string{
		"internal/web/static/crm",
		filepath.Join("..", "internal", "web", "static", "crm"),
	}
	for _, dir := range candidates {
		if isCRMBuildDir(dir) {
			abs, err := filepath.Abs(dir)
			if err != nil {
				return dir, true, nil
			}
			return abs, true, nil
		}
	}
	if _, err := fs.Stat(crmAssets, "static/crm/index.html"); err != nil {
		return "", false, fmt.Errorf("crm ui: run make build-crm")
	}
	return "", false, nil
}

func isCRMBuildDir(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "index.html"))
	if err != nil || st.IsDir() {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "_next"))
	return err == nil
}

func crmRelativePath(raw string) (string, bool) {
	rel := strings.TrimPrefix(raw, "/")
	if rel == "" {
		return "index.html", true
	}
	if strings.Contains(rel, "..") {
		return "", false
	}
	if strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	return rel, true
}

func serveCRMFile(c *gin.Context, fsys fs.FS, rel string) {
	data, err := fs.ReadFile(fsys, rel)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(rel))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	if strings.HasSuffix(rel, ".html") {
		ctype = "text/html; charset=utf-8"
	}
	c.Header("Content-Type", ctype)
	if strings.HasSuffix(rel, ".html") {
		c.Header("Cache-Control", "no-cache")
	} else if strings.HasPrefix(rel, "_next/static/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "public, max-age=3600")
	}
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(data)
}
