package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// setToast fires a client-side toast via the HX-Trigger header.
func setToast(c *gin.Context, tone, message string) {
	mergeHXTrigger(c, map[string]any{
		"toast": map[string]string{
			"tone":    tone,
			"message": message,
		},
	})
}

// mergeHXTrigger merges additional keys into the HX-Trigger JSON header.
func mergeHXTrigger(c *gin.Context, extra map[string]any) {
	existing := c.Writer.Header().Get("HX-Trigger")
	base := map[string]any{}
	if existing != "" {
		_ = json.Unmarshal([]byte(existing), &base)
	}
	for k, v := range extra {
		base[k] = v
	}
	b, err := json.Marshal(base)
	if err != nil {
		return
	}
	c.Header("HX-Trigger", string(b))
}

// renderHTMXError writes a reusable inline error partial for failed HTMX
// requests.
func (rd *Renderer) renderHTMXError(c *gin.Context, status int, message string) {
	c.Status(status)
	rd.RenderPartial(c, "inline_error", map[string]string{
		"message": message,
	})
}

// renderRetryState writes an inline retry banner partial.
func (rd *Renderer) renderRetryState(c *gin.Context, message, url, target, swap string) {
	c.Status(http.StatusBadGateway)
	rd.RenderPartial(c, "retry_state", map[string]string{
		"message": message,
		"url":     url,
		"target":  target,
		"swap":    swap,
	})
}

// escapeJSONString is a minimal escaper for toast messages in headers.
func escapeJSONString(s string) string {
	return strings.ReplaceAll(s, `"`, `'`)
}
