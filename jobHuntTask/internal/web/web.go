// Package web renders server-side HTML pages from html/template files
// embedded into the binary. It is intentionally tiny:
//
//   - one Renderer holds N pre-parsed page templates
//   - each page is composed of (base layout + all partials + that page)
//   - HTMX requests get the inner content block; full requests get the
//     full base layout — single rendering path, branch on a header
//
// Static assets are served from the same embed.FS so deployments stay
// single-binary.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*/*.html static/css/*.css static/js/*.js
var assets embed.FS

// crmPath is the in-app CRM UI route (embedded static export on the Go server).
var crmPath = "/crm/"

// SetCRMPath configures the sidebar CRM link (default /crm/).
func SetCRMPath(path string) {
	if path != "" {
		crmPath = path
		if !strings.HasPrefix(crmPath, "/") {
			crmPath = "/" + crmPath
		}
		if !strings.HasSuffix(crmPath, "/") {
			crmPath += "/"
		}
	}
}

// PageData is the canonical context every page receives. Pages may extend
// it with their own structs, but Title/Active/Now are always present so
// the layout can render uniformly.
type PageData struct {
	Title  string
	Active string // sidebar nav highlight, e.g. "dashboard", "tasks"
	Now    time.Time
	Data   any // page-specific payload
}

// Renderer holds a fully-parsed template per page plus a dedicated
// "partials" tree (layout + every partial, no page bodies) used to
// render HTMX fragments by name.
type Renderer struct {
	pages    map[string]*template.Template
	partials *template.Template
}

// New parses every page in templates/pages and returns a ready Renderer.
// Returns an error rather than panicking so the caller decides what to do.
func New() (*Renderer, error) {
	pagePaths, err := fs.Glob(assets, "templates/pages/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob pages: %w", err)
	}
	layoutPaths, err := fs.Glob(assets, "templates/layout/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob layouts: %w", err)
	}
	partialPaths, err := fs.Glob(assets, "templates/partials/*.html")
	if err != nil {
		return nil, fmt.Errorf("glob partials: %w", err)
	}

	pages := make(map[string]*template.Template, len(pagePaths))
	for _, pagePath := range pagePaths {
		name := strings.TrimSuffix(filepath.Base(pagePath), ".html")

		t := template.New(filepath.Base(layoutPaths[0])).Funcs(funcMap())

		all := make([]string, 0, len(layoutPaths)+len(partialPaths)+1)
		all = append(all, layoutPaths...)
		all = append(all, partialPaths...)
		all = append(all, pagePath)

		t, err := t.ParseFS(assets, all...)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", pagePath, err)
		}
		pages[name] = t
	}

	// A separate tree with just layout + partials, used for partial
	// rendering. Keeping it independent of the page templates avoids
	// stomping on `content` block redefinitions.
	partials := template.New("partials").Funcs(funcMap())
	all := append([]string{}, layoutPaths...)
	all = append(all, partialPaths...)
	if len(all) > 0 {
		partials, err = partials.ParseFS(assets, all...)
		if err != nil {
			return nil, fmt.Errorf("parse partials: %w", err)
		}
	}

	return &Renderer{pages: pages, partials: partials}, nil
}

// Render writes the named page. If the request carries HX-Request: true,
// only the page's `content` block is written so HTMX can swap it into
// the existing layout. Otherwise the full base layout is rendered.
func (r *Renderer) Render(c *gin.Context, name string, pd PageData) {
	t, ok := r.pages[name]
	if !ok {
		c.String(http.StatusInternalServerError, "template not found: %s", name)
		return
	}
	if pd.Now.IsZero() {
		pd.Now = time.Now()
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.GetHeader("HX-Request") == "true" {
		// Push the new URL so back/forward work; partial swap below.
		c.Header("HX-Push-Url", c.Request.URL.Path)
		if err := t.ExecuteTemplate(c.Writer, "content", pd); err != nil {
			c.String(http.StatusInternalServerError, "render fragment: %v", err)
		}
		return
	}
	if err := t.ExecuteTemplate(c.Writer, "base.html", pd); err != nil {
		c.String(http.StatusInternalServerError, "render page: %v", err)
	}
}

// RenderPartial writes a single named partial. Used by HTMX endpoints
// that return one card fragment, not a whole page. The named template
// must be `{{define "name"}}` somewhere under templates/partials.
func (r *Renderer) RenderPartial(c *gin.Context, name string, data any) {
	if r.partials == nil {
		c.String(http.StatusInternalServerError, "no partials registered")
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := r.partials.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.String(http.StatusInternalServerError, "render partial %q: %v", name, err)
	}
}

// MountStatic mounts /static under the given Gin engine, serving the
// embedded assets directly. No filesystem leak surface.
func MountStatic(r *gin.Engine) error {
	sub, err := fs.Sub(assets, "static")
	if err != nil {
		return fmt.Errorf("sub static: %w", err)
	}
	r.StaticFS("/static", http.FS(sub))
	return nil
}

// ---------------------------------------------------------------------------
// Template helpers
// ---------------------------------------------------------------------------

func funcMap() template.FuncMap {
	m := template.FuncMap{
		"dict":    dict,
		"safeURL": func(s string) template.URL { return template.URL(s) },
		"year":    func() int { return time.Now().Year() },
		"hasKey":  hasKey,
		"crmURL":  func() string { return crmPath },
	}
	for k, v := range tasksFuncMap() {
		m[k] = v
	}
	for k, v := range weeklyFuncMap() {
		m[k] = v
	}
	for k, v := range analyticsFuncMap() {
		m[k] = v
	}
	return m
}

// dict turns a flat (key, value, key, value, ...) list into a map so
// partials can be called with named parameters. Returns an error rather
// than panicking; html/template will surface that nicely during dev.
func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments (%d)", len(values))
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key at index %d is not a string", i)
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// hasKey reports whether a dict has the given key — used in partials to
// gate optional rendering blocks.
func hasKey(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	_, ok := m[key]
	return ok
}
